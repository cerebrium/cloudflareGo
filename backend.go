package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
)

// Interfaces
type locationString struct {
	City string
}

// example 404 function
func notFound(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotFound)
	w.Write([]byte(`{"message": "404"}`))
}

// cloudflare function
func getCloudflare(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// check if the size is too large
	r.Body = http.MaxBytesReader(w, r.Body, 1048576)

	// get the body from the request
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	// set the stype of the response we are looking for
	var loc locationString
	err := dec.Decode(&loc)
	if err != nil {
		var syntaxError *json.SyntaxError
		var unmarshalTypeError *json.UnmarshalTypeError

		switch {
		// syntax error case
		case errors.As(err, &syntaxError):
			msg := fmt.Sprintf("Request body contains badly formed JSON (at position %d)", syntaxError.Offset)
			http.Error(w, msg, http.StatusBadRequest)

			// case of decode returning an EOF because of bad json syntax
		case errors.Is(err, io.ErrUnexpectedEOF):
			msg := fmt.Sprintf("Request body contains badly-formed JSON")
			http.Error(w, msg, http.StatusBadRequest)

			// catch errors where types are being messed up
		case errors.As(err, &unmarshalTypeError):
			msg := fmt.Sprintf("Request body contains an invalid value for the %q field (at position %d)", unmarshalTypeError.Field, unmarshalTypeError.Offset)
			http.Error(w, msg, http.StatusBadRequest)

			// if there are extra unexpected fields in the body it throws an error
		case strings.HasPrefix(err.Error(), "json: unknown field "):
			fieldName := strings.TrimPrefix(err.Error(), "json: unknown field ")
			msg := fmt.Sprintf("Request body contains and unknown field %s", fieldName)
			http.Error(w, msg, http.StatusBadRequest)

			// if the body is empty it returns an EOF
		case errors.Is(err, io.EOF):
			msg := "Request body must not be empty"
			http.Error(w, msg, http.StatusBadRequest)

			// if the body is too long, handle that
		case err.Error() == "http: request body too large":
			msg := "Request body must not be larger than 1MB"
			http.Error(w, msg, http.StatusRequestEntityTooLarge)

			// default to sending the error and a 500
		default:
			log.Println(err.Error())
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		}
		return
	}

	// If the request body only contained a single JSON object this will return an io.EOF error. So if we get anything else,
	// we know that there is additional data in the request body.
	err = dec.Decode(&struct{}{})
	if err != io.EOF {
		msg := "Requset body must conatin a single JSON object"
		http.Error(w, msg, http.StatusBadRequest)
		return
	}
}

func main() {
	// allow for env variables
	err := godotenv.Load()

	// handle error of loading
	if err != nil {
		log.Fatal(err)
	}

	// declares resonses as a variabale to be handeled
	r := mux.NewRouter()

	// set each metehod of response to be dealt with by the correct function
	r.HandleFunc("/cloudflare", getCloudflare).Methods("POST")
	r.HandleFunc("/", notFound)

	// this sets the server to 8080
	log.Fatal(http.ListenAndServe(":8080", r))
}

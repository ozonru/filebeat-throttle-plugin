package main

import (
	"net/http"
)

func GetHandler(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "config.yml")
}

func main() {
	http.HandleFunc("/policy", GetHandler)
	http.ListenAndServe(":8080", nil)
}

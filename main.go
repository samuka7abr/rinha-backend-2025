package main

import (
	"fmt"
	"log"
	"net/http"
)

func helloHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "Olá, Rinha de Backend!")
}

func main() {
	http.HandleFunc("/", helloHandler)

	port := "8080"
	fmt.Printf("Servidor iniciado na porta %s\n", port)

	log.Fatal(http.ListenAndServe(":"+port, nil))
}
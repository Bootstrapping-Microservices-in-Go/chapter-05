package main

import (
	"fmt"
	"net/http"
	"os"
)

func main() {
	port := os.Getenv(`PORT`)
	http.ListenAndServe(fmt.Sprintf(":%s", port))
}

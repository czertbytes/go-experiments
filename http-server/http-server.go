package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"time"
)

func main() {

	l, err := net.Listen("tcp", ":8098")
	if err != nil {
		log.Fatal(err)
	}

	handler := http.NewServeMux()
	handler.HandleFunc("/stop", func(w http.ResponseWriter, r *http.Request) {
		if err = l.Close(); err != nil {
			log.Fatal(err)
		}
	})

	handler.HandleFunc("/hello", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("hello"))
	})

	go func() {
		if err := http.Serve(l, handler); err == nil {
			log.Fatal(err)
		}
		fmt.Println("server is done")
	}()

	log.Println("Waiting for R1")
	time.Sleep(3 * time.Second)
	log.Println("R1")
	r1, err := http.Get("http://localhost:8098/hello")
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("R1: %d\n", r1.StatusCode)

	log.Println("Waiting for R2")
	time.Sleep(3 * time.Second)
	log.Println("R2")
	r2, err := http.Get("http://localhost:8098/hello")
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("R2: %d\n", r2.StatusCode)

	time.Sleep(2 * time.Second)
	http.Get("http://localhost:8098/stop")

	log.Println("server stopped")
}

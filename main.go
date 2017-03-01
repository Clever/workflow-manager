package main

import (
	"flag"
	"log"

	"github.com/Clever/workflow-manager/gen-go/server"
)

func main() {
	addr := flag.String("addr", ":8080", "Address to listen at")
	flag.Parse()

	wm := WorkflowManager{}
	s := server.New(wm, *addr)

	if err := s.Serve(); err != nil {
		log.Fatal(err)
	}

	log.Println("workflow-manager exited without error")
}

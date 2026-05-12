package health

import (
	"fmt"
	"log"
	"net/http"
)

func Init() {
	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "OK")
	})
	go func() {
		err := http.ListenAndServe(":8008", nil)
		if err != nil {
			log.Fatal("creating /healthz endpoint", err)
		}
	}()
}

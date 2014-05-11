package main

import (
	"fmt"
	"github.com/bmizerany/pat"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"raid5"
)

var (
	data1, data2, parity string //pathnames to the directories
)

//we just use the directories in the temp dir since this is a test
//progarm
func init() {
	var err error
	data1, err = ioutil.TempDir("", "raid5")
	if err != nil {
		log.Fatalf("creating data dir: %v", err)
	}
	data2, err = ioutil.TempDir("", "raid5")
	if err != nil {
		log.Fatalf("creating data dir2: %v", err)
	}
	parity, err = ioutil.TempDir("", "raid5")
	if err != nil {
		log.Fatalf("creating parity dir: %v", err)
	}
}

func putData(w http.ResponseWriter, req *http.Request) {
	n := req.URL.Query().Get(":name")
	obj, err := raid5.CreateFile(data1, data2, parity, n)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		io.WriteString(w, fmt.Sprintf("%s", err))
		return
	}
	if req.ContentLength == 0 {
		obj.Close()
		io.WriteString(w, "ok")
		return
	}
	buffer, err := ioutil.ReadAll(req.Body)
	req.Body.Close()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		io.WriteString(w, "failed to read the body supplied")
		return
	}
	_, _, err = obj.WriteAndClose(buffer)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		io.WriteString(w, fmt.Sprintf("%v", err))
		return
	}

	log.Printf("wrote: %s\n", n)
	//everything is ok
	io.WriteString(w, "ok")
}

func readData(w http.ResponseWriter, req *http.Request) {
	n := req.URL.Query().Get(":name")
	obj, err := raid5.OpenFile(data1, data2, parity, n)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		io.WriteString(w, fmt.Sprintf("%s", err))
		return
	}
	if obj.Size() == 0 {
		return
	}
	//could get fancy and do this in blobs, but not bothering to
	//do that extra looping
	buffer := make([]byte, obj.Size())
	_, err = obj.ReadFile(buffer, 0)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		io.WriteString(w, fmt.Sprintf("%v", err))
		return
	}
	w.Write(buffer)
	log.Printf("finished writing %s to client", n)
}

func main() {
	m := pat.New()
	m.Get("/raid5/:name", http.HandlerFunc(readData))
	m.Put("/raid5/:name", http.HandlerFunc(putData))
	http.Handle("/", m)

	defer func() {
		os.RemoveAll(data1)
		os.RemoveAll(data2)
		os.RemoveAll(parity)
	}()

	log.Printf("data directories for the server:\n%s\n%s\n%s\n",
		data1, data2, parity)
	log.Fatalf("returned from listen and serve",
		http.ListenAndServe(":8080", nil))
}

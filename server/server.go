package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"strings"

	"github.com/go-k8s-metadata/diff"

	"github.com/go-k8s-metadata/tika"
	"github.com/gorilla/mux"
)

// Flags requiring input.
const (
	parse    = "parse"
	detect   = "detect"
	language = "language"
	meta     = "meta"
)

// Informational flags which don't require input.
const (
	version   = "version"
	parsers   = "parsers"
	mimeTypes = "mimetypes"
	detectors = "detectors"
)

var (
	TikaClient *tika.Client
	metaField  string
	recursive  bool = false
)
var ResPayloads []ResPayload

type ResPayload struct {
	Left   string  `json:"left_file"`
	Right  string  `json:"right_file"`
	Result float64 `json:"difference_percentage"`
}

func uploadHandler(w http.ResponseWriter, r *http.Request) {
	var fileorg io.Reader
	var rebfile io.Reader
	leftfile, _, err := r.FormFile("left_file")
	if err != nil {
		//http.Error(w, err.Error(), http.StatusBadRequest)
		respondWithError(w, http.StatusBadRequest, "Invalid request payload")

		return
	}

	defer leftfile.Close()
	f, err := ioutil.ReadAll(leftfile)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	fileorg = bytes.NewReader(f)
	if err != nil {
		fmt.Fprintf(w, "Unable to create the file for writing. Check your write access privilege")
		return
	}
	porg, err := processreq("parse", fileorg)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	rightfile, _, err := r.FormFile("right_file")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	defer rightfile.Close()
	fr, err := ioutil.ReadAll(rightfile)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	rebfile = bytes.NewReader(fr)
	if err != nil {
		fmt.Fprintf(w, "Unable to create the file for writing. Check your write access privilege")
		return
	}
	preb, err := processreq("parse", rebfile)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	d := diff.CharacterDiffEx(porg, preb)

	diftotal := float64(len(d))
	total := float64(len(porg))

	p := diftotal / total * 100

	var a ResPayload
	if p > 100 {
		a.Result = 100
	} else {
		a.Result = p
	}

	//rjson := json.NewEncoder(w).Encode(a)

	respondWithJSON(w, http.StatusOK, a)

}

func (p *ResPayload) createProduct() error {

	return nil
}

func respondWithError(w http.ResponseWriter, code int, message string) {
	respondWithJSON(w, code, map[string]string{"error": message})
}
func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	response, _ := json.Marshal(payload)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(response)
}
func returnSingleResPayload(w http.ResponseWriter, r *http.Request) {
	var a ResPayload
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&a); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}
	defer r.Body.Close()
	if err := a.createProduct(); err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, a.Left)
}

func Start(port string, Tika *tika.Client) {
	fmt.Println("start api")
	TikaClient = Tika
	myRouter := mux.NewRouter().StrictSlash(true)
	myRouter.HandleFunc("/visual_compare", uploadHandler)

	go func() {
		log.Fatal(http.ListenAndServe(":"+port, myRouter))

	}()
}
func processreq(action string, file io.Reader) (string, error) {
	if TikaClient == nil {
		return "", errors.New("no client")
	}

	switch action {
	default:
		flag.Usage()
		return "", fmt.Errorf("error: invalid action %q", action)
	case parse:
		if recursive {
			bs, err := TikaClient.ParseRecursive(context.Background(), file)
			if err != nil {
				return "", err
			}
			return strings.Join(bs, "\n"), nil
		}
		return TikaClient.Parse(context.Background(), file)
	case detect:
		return TikaClient.Detect(context.Background(), file)
	case language:
		return TikaClient.Language(context.Background(), file)
	case meta:
		if metaField != "" {
			return TikaClient.MetaField(context.Background(), file, metaField)
		}
		if recursive {
			mr, err := TikaClient.MetaRecursive(context.Background(), file)
			if err != nil {
				return "", err
			}
			bytes, err := json.MarshalIndent(mr, "", "  ")
			if err != nil {
				return "", err
			}
			return string(bytes), nil
		}
		return TikaClient.Meta(context.Background(), file)
	case version:
		return TikaClient.Version(context.Background())
	case parsers:
		p, err := TikaClient.Parsers(context.Background())
		if err != nil {
			return "", err
		}
		bytes, err := json.MarshalIndent(p, "", "  ")
		if err != nil {
			return "", err
		}
		return string(bytes), nil
	case mimeTypes:
		mt, err := TikaClient.MIMETypes(context.Background())
		if err != nil {
			return "", err
		}
		bytes, err := json.MarshalIndent(mt, "", "  ")
		if err != nil {
			return "", err
		}
		return string(bytes), nil
	case detectors:
		d, err := TikaClient.Detectors(context.Background())
		if err != nil {
			return "", err
		}
		bytes, err := json.MarshalIndent(d, "", "  ")
		if err != nil {
			return "", err
		}
		return string(bytes), nil
	}
}

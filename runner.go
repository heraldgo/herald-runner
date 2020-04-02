package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"path/filepath"
	"strings"

	"github.com/heraldgo/heraldd/util"
)

type runner struct {
	util.HTTPServer
	exeGit util.ExeGit
	secret string
}

func (r *runner) getOutputPath(pathOrigin string) string {
	if filepath.IsAbs(pathOrigin) {
		return pathOrigin
	}
	return filepath.Join(r.exeGit.WorkRunDir(), pathOrigin)
}

func (r *runner) validateSignature(req *http.Request, body []byte) error {
	if req.Method != "POST" {
		return fmt.Errorf("Only POST request allowed")
	}

	sigHeader := req.Header.Get("X-Herald-Signature")
	signature, err := hex.DecodeString(sigHeader)
	if err != nil {
		return fmt.Errorf("Invalid X-Herald-Signature: %s", sigHeader)
	}
	key := []byte(r.secret)

	if !util.ValidateMAC(body, signature, key) {
		return fmt.Errorf("Signature validation Error")
	}
	return nil
}

func (r *runner) respondSingle(w http.ResponseWriter, result map[string]interface{}) {
	w.Header().Set("Content-Type", "application/json")

	delete(result, "file")
	resultJSON, err := json.Marshal(result)
	if err != nil {
		w.Write([]byte(fmt.Sprintf(`{"error":"Generate json result error: %s"}`, err)))
	} else {
		w.Write(resultJSON)
	}
}

var quoteEscaper = strings.NewReplacer("\\", "\\\\", `"`, "\\\"")

func escapeQuotes(s string) string {
	return quoteEscaper.Replace(s)
}

func (r *runner) writeResultPart(mpw *multipart.Writer, result map[string]interface{}) {
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition",
		fmt.Sprintf(`form-data; name="%s"`, "result"))
	h.Set("Content-Type", "application/json")
	rpw, err := mpw.CreatePart(h)
	if err != nil {
		r.Errorf("Create multipart form field error: %s", err)
		return
	}

	delete(result, "file")
	resultJSON, err := json.Marshal(result)
	if err != nil {
		rpw.Write([]byte(fmt.Sprintf(`{"error":"Generate json result error: %s"}`, err)))
	} else {
		rpw.Write(resultJSON)
	}
}

func (r *runner) writeFilePart(mpw *multipart.Writer, name, filePath string) {
	sha256Sum, err := util.SHA256SumFile(filePath)
	if err != nil {
		r.Errorf("Get sha256 checksum error: %s", err)
		return
	}

	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition",
		fmt.Sprintf(`form-data; name="%s"; filename="%s"; sha256sum="%s"`,
			escapeQuotes(name), escapeQuotes(filepath.Base(filePath)),
			hex.EncodeToString(sha256Sum)))
	h.Set("Content-Type", "application/octet-stream")

	fpw, err := mpw.CreatePart(h)
	if err != nil {
		r.Errorf("Create multipart form field error: %s", err)
		return
	}

	f, err := os.Open(filePath)
	if err != nil {
		r.Errorf("Output file open error: %s", err)
		return
	}
	defer f.Close()

	_, err = io.Copy(fpw, f)
	if err != nil {
		r.Errorf("Multipart copy file error: %s", err)
		return
	}

	r.Infof("Output file added successfully: %s", filePath)
}

func (r *runner) respondMultiple(w http.ResponseWriter, result map[string]interface{}) {
	resultFiles, _ := util.GetMapParam(result, "file")

	mpWriter := multipart.NewWriter(w)

	w.Header().Set("Content-Type", mpWriter.FormDataContentType())

	r.writeResultPart(mpWriter, result)

	for name, filePath := range resultFiles {
		fp, ok := filePath.(string)
		if !ok {
			r.Warnf("File value must be string of file path: %v", filePath)
			continue
		}
		fnOutput := r.getOutputPath(fp)
		r.writeFilePart(mpWriter, name, fnOutput)
	}

	mpWriter.Close()
}

func (r *runner) processExecution(w http.ResponseWriter, req *http.Request, body []byte) {
	r.Infof("Start to execute...")

	bodyMap, err := util.JSONToMap(body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(fmt.Sprintf("Request body error: %s", err)))
		return
	}

	result := r.exeGit.Execute(bodyMap)
	r.Debugf("Execute result: %#v", result)

	fileMap, _ := result["file"].(map[string]interface{})

	if len(fileMap) == 0 {
		r.respondSingle(w, result)
	} else {
		r.respondMultiple(w, result)
	}
}

func (r *runner) Run(ctx context.Context) {
	r.ValidateFunc = r.validateSignature
	r.ProcessFunc = r.processExecution

	r.Start()
	defer r.Stop()

	<-ctx.Done()
}

// SetLogger will set logger for both HTTPServer and exeGit
func (r *runner) SetLogger(logger interface{}) {
	r.HTTPServer.SetLogger(logger)
	r.exeGit.SetLogger(logger)
}

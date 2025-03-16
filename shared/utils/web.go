package utils

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/itchan-dev/itchan/shared/errors"
)

func WriteErrorAndStatusCode(w http.ResponseWriter, err error) {
	if e, ok := err.(*errors.ErrorWithStatusCode); ok {
		http.Error(w, err.Error(), e.StatusCode)
		return
	}
	// default error is 500
	http.Error(w, err.Error(), http.StatusInternalServerError)
}

func GetIP(r *http.Request) (string, error) {
	//Get IP from the X-REAL-IP header
	ip := r.Header.Get("X-REAL-IP")
	netIP := net.ParseIP(ip)
	if netIP != nil {
		return ip, nil
	}

	//Get IP from X-FORWARDED-FOR header
	ips := r.Header.Get("X-FORWARDED-FOR")
	splitIps := strings.Split(ips, ",")
	for _, ip := range splitIps {
		netIP := net.ParseIP(ip)
		if netIP != nil {
			return ip, nil
		}
	}

	//Get IP from RemoteAddr
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		log.Printf(err.Error())
		return "", err
	}
	netIP = net.ParseIP(ip)
	if netIP != nil {
		return ip, nil
	}
	return "", fmt.Errorf("No valid ip found")
}

func DecodeValidate(r io.ReadCloser, body any) error {
	if err := json.NewDecoder(r).Decode(body); err != nil {
		log.Printf(err.Error())
		return &errors.ErrorWithStatusCode{Message: "Body is invalid json", StatusCode: 400}
	}
	validate := validator.New(validator.WithRequiredStructEnabled())
	if err := validate.Struct(body); err != nil {
		log.Printf(err.Error())
		return &errors.ErrorWithStatusCode{Message: "Required fields missing", StatusCode: 400}
	}
	return nil
}

func Decode(r io.ReadCloser, body any) error {
	if err := json.NewDecoder(r).Decode(body); err != nil {
		log.Printf(err.Error())
		return &errors.ErrorWithStatusCode{Message: "Body is invalid json", StatusCode: 400}
	}
	return nil
}

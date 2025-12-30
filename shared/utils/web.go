package utils

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/itchan-dev/itchan/shared/errors"
	"github.com/itchan-dev/itchan/shared/logger"
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
	splitIps := strings.SplitSeq(ips, ",")
	for ip := range splitIps {
		netIP := net.ParseIP(ip)
		if netIP != nil {
			return ip, nil
		}
	}

	//Get IP from RemoteAddr
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		logger.Log.Error("failed to split host port", "error", err)
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
		logger.Log.Error("failed to decode json", "error", err)
		return &errors.ErrorWithStatusCode{Message: "Body is invalid json", StatusCode: 400}
	}
	validate := validator.New(validator.WithRequiredStructEnabled())
	if err := validate.Struct(body); err != nil {
		logger.Log.Error("validation failed", "error", err)
		return &errors.ErrorWithStatusCode{Message: "Required fields missing", StatusCode: 400}
	}
	return nil
}

func Decode(r io.ReadCloser, body any) error {
	if err := json.NewDecoder(r).Decode(body); err != nil {
		logger.Log.Error("failed to decode json", "error", err)
		return &errors.ErrorWithStatusCode{Message: "Body is invalid json", StatusCode: 400}
	}
	return nil
}

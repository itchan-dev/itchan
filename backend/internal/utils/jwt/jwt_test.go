package jwt

import (
	"strconv"
	"testing"
	"time"

	"github.com/itchan-dev/itchan/shared/domain"
)

var secretKey string = "testJwtKey"
var user domain.User = domain.User{Email: "test@mail.ru", PassHash: []byte("testpass"), Id: 1}

func TestDecodeTokenCorrect(t *testing.T) {
	jwt := New(secretKey, time.Duration(10*1000000000))
	token, err := jwt.NewToken(&user)
	if err != nil {
		t.Errorf(err.Error())
	}

	claims, err := jwt.DecodeToken(token)
	if err != nil {
		t.Errorf(err.Error())
	}
	if uid := claims["uid"].(float64); uid != 1 {
		t.Errorf("%s != 1", strconv.Itoa(int(uid)))
	}
	if email := claims["email"]; email != "test@mail.ru" {
		t.Errorf("%s != %s", email, "test@mail.ru")
	}
}

func TestDecodeTokenExpired(t *testing.T) {
	jwt := New(secretKey, time.Duration(0))
	token, err := jwt.NewToken(&user)
	if err != nil {
		t.Errorf(err.Error())
	}

	_, err = jwt.DecodeToken(token)
	if err == nil {
		t.Errorf("We shouldn't decode expired token")
	}
}

func TestDecodeTokenInvalidSecretKey(t *testing.T) {
	token, err := New(secretKey, time.Duration(10*1000000000)).NewToken(&user)
	if err != nil {
		t.Errorf(err.Error())
	}

	_, err = New("invalidSecret", time.Duration(10*1000000000)).DecodeToken(token)
	if err == nil {
		t.Errorf("We shouldn't decode token with invalid secret")
	}
}

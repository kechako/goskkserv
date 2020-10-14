package skkserv

import (
	"errors"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/encoding/unicode"
)

type Encoding string

const (
	UTF8     Encoding = "utf-8"
	EUCJP    Encoding = "euc-jp"
	ShiftJIS Encoding = "sjis"
)

func ParseEncoding(s string) (Encoding, error) {
	enc := Encoding(s)
	switch enc {
	case UTF8, EUCJP, ShiftJIS:
		return enc, nil
	}

	return "", errors.New("invalid encoding")
}

func (enc Encoding) encoding() encoding.Encoding {
	switch enc {
	case UTF8:
		return unicode.UTF8
	case EUCJP:
		return japanese.EUCJP
	case ShiftJIS:
		return japanese.ShiftJIS
	default:
		return unicode.UTF8
	}
}

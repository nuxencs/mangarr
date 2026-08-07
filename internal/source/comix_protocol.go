package source

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

// comixFrontendBuild identifies the frontend bundle that defines the codec profile below.
const comixFrontendBuild = "35595e3de3c99889c1aa70"

const (
	comixAPIPath                   = "/api/v1"
	comixTokenParameter            = "_"
	comixResponseEncryptionHeader  = "x-enc"
	comixResponseEncryptionEnabled = "1"
	comixCodecAlphabetSize         = 1 << 8

	comixStage1Table = "gbicCvAMzfcXEtGAyjvvhmb2yCWzWhjqcxXZ7ZhpzANOzoQLo3nuPZ2vK9dkb9hJExC0Vni/hdQBceI+mw611gkhQFjBuf4bJg1TxYqM+SL4YDqtwjxiGSdeH7so7Fn1HiRo37Z+RNvl44twXWVhomtMjw+8bemfmv9XEXr7mS82MxaCOJZRR0oHd9PLI5O+gyBGT6hcLoduNa7yCObVVCk3bFWsoD+xcqTrBcP6dNJN/NB1Br2QGhSN2snHAqeRNKVFQiyeAFLPSKGwY8aq9EPgsi17qd4ywPMxiH8w6N1qX1tLKtzhOeemHWeJQfFQ5H23q7qSlJUcjgTEl3x2/Q=="
	comixStage1Key   = "rafYl4oSAKQX+GYoic9oW4iGwiYpZzs0"
	comixStage1Seed  = 189
	comixStage2Table = "2lQehmgyYFAoWUi0haazZqHy5zZ34NN+VzlfsoB2Y1yY0IuMLjgVcV2xt8t4moH+AP0NMJ5qekW7DFIHEWKkOgIBIMhDdA8lbM6iHKjDlq6IChpb3CnA9NmsvQW/afdt1SfJjTdwcvpKqunCJLxBFmXX9hecm6tGb+HRxD7BC3njoxPxgnX5pdKP1IMSkd4/O3NRfZSE6DVLG2s9uexaipA05cpJzE8Qkv/z5jzHAwlEWOLd3yxA+0cvVbpOoJPFGc8f1lb4vu2HUxjuuEwEQk0GsPCVnyKvfOoh9TG2YYmZLV4I67UU2NsrrakqZ47k/O+ne25/DjPGZCMdnZcmzQ=="
	comixStage2Key   = "2USAq+VTo5ht4bQn+K9DUcpUQRTtrB56"
	comixStage2Seed  = 133
	comixStage3Table = "+mhJSFwzaV+PQPDyKp2scO/S9SdFsy/7e56UWT8XHbK3E2+19nEPwfwOgE9uVCaDtOAWTobCZX+cBCXlIbBqyDyQB1beKLspW6kGPhBCV9x0jf0KUeFhHjmlMf7qMFIB41PfDFprZ3bJiK4YxrZDv+K6dcwJmggVO8f5ktrXTM0cZL4fer0SpnkbvNajPbHxfuTz5lVEBarOI4rdc+2V6zTsjpfQYjgN1MMr6EvA6eehN6dQ1bgUogt9rZOBbQBeNnLYY00uZqSoJBnFi5gthCJsWF33ykosn9v/9KB8udMCz0YRYImrA4VHr5mMgpH4xDXLeEHRd5vZOiAalofuMg=="
	comixStage3Key   = "yNHlokVEnuecesDrB/lDhVuUNiheWc3a47VtkwZ2ENg="
	comixStage3Seed  = 32
)

type comixParams map[string]any

type comixCodecStage struct {
	table   []byte
	inverse [comixCodecAlphabetSize]byte
	key     []byte
	seed    byte
}

type comixCodec struct {
	stages []comixCodecStage
}

func newComixCodec() (comixCodec, error) {
	definitions := []struct {
		table string
		key   string
		seed  byte
	}{
		{table: comixStage1Table, key: comixStage1Key, seed: comixStage1Seed},
		{table: comixStage2Table, key: comixStage2Key, seed: comixStage2Seed},
		{table: comixStage3Table, key: comixStage3Key, seed: comixStage3Seed},
	}

	codec := comixCodec{stages: make([]comixCodecStage, 0, len(definitions))}
	for i, definition := range definitions {
		table, err := base64.StdEncoding.DecodeString(definition.table)
		if err != nil {
			return comixCodec{}, fmt.Errorf("decoding Comix stage %d table: %w", i+1, err)
		}
		if len(table) != comixCodecAlphabetSize {
			return comixCodec{}, fmt.Errorf("Comix stage %d table has length %d, want %d", i+1, len(table), comixCodecAlphabetSize)
		}

		key, err := base64.StdEncoding.DecodeString(definition.key)
		if err != nil {
			return comixCodec{}, fmt.Errorf("decoding Comix stage %d key: %w", i+1, err)
		}
		if len(key) == 0 {
			return comixCodec{}, fmt.Errorf("Comix stage %d key is empty", i+1)
		}

		stage := comixCodecStage{table: table, key: key, seed: definition.seed}
		var seen [256]bool
		for input, output := range table {
			if seen[output] {
				return comixCodec{}, fmt.Errorf("Comix stage %d table is not a permutation", i+1)
			}
			seen[output] = true
			stage.inverse[output] = byte(input)
		}
		codec.stages = append(codec.stages, stage)
	}

	return codec, nil
}

func (c comixCodec) token(rawURL string, params comixParams) (string, error) {
	input, err := canonicalComixRequest(rawURL, params)
	if err != nil {
		return "", err
	}

	encoded := []byte(input)
	for _, stage := range c.stages {
		encoded = stage.encode(encoded)
	}

	return base64.RawURLEncoding.EncodeToString(encoded), nil
}

func (c comixCodec) decodeResponse(reader io.Reader, encrypted bool, destination any) error {
	body, err := io.ReadAll(reader)
	if err != nil {
		return fmt.Errorf("reading Comix response: %w", err)
	}

	if encrypted {
		var envelope struct {
			Encoded string `json:"e"`
		}
		if err := json.Unmarshal(body, &envelope); err != nil {
			return fmt.Errorf("decoding Comix encrypted envelope: %w", err)
		}
		if envelope.Encoded == "" {
			return fmt.Errorf("decoding Comix encrypted envelope: missing e field")
		}

		body, err = base64.RawURLEncoding.DecodeString(envelope.Encoded)
		if err != nil {
			return fmt.Errorf("decoding Comix encrypted payload: %w", err)
		}
		for i := len(c.stages) - 1; i >= 0; i-- {
			body = c.stages[i].decode(body)
		}
		if !utf8.Valid(body) {
			return fmt.Errorf("decoding Comix encrypted payload: invalid UTF-8")
		}
	}

	var response struct {
		Status string          `json:"status"`
		Result json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return fmt.Errorf("decoding Comix response JSON: %w", err)
	}
	if response.Status != "ok" {
		return fmt.Errorf("decoding Comix response: unexpected status %q", response.Status)
	}
	if len(response.Result) == 0 {
		return fmt.Errorf("decoding Comix response: missing result")
	}
	if err := json.Unmarshal(response.Result, destination); err != nil {
		return fmt.Errorf("decoding Comix result: %w", err)
	}

	return nil
}

func (s comixCodecStage) encode(input []byte) []byte {
	output := make([]byte, len(input))
	previous := s.seed
	for i, value := range input {
		output[i] = s.table[value^s.key[i%len(s.key)]^previous]
		previous = output[i]
	}

	return output
}

func (s comixCodecStage) decode(input []byte) []byte {
	output := make([]byte, len(input))
	previous := s.seed
	for i, value := range input {
		output[i] = s.inverse[value] ^ s.key[i%len(s.key)] ^ previous
		previous = value
	}

	return output
}

func canonicalComixRequest(rawURL string, params comixParams) (string, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("parsing Comix request URL: %w", err)
	}

	path := parsed.Path
	if path == comixAPIPath {
		path = ""
	} else if strings.HasPrefix(path, comixAPIPath+"/") {
		path = strings.TrimPrefix(path, comixAPIPath)
	}
	if !isComixProtectedPath(path) {
		return "", fmt.Errorf("unsupported Comix token path %q", path)
	}

	pairs := make([]string, 0, len(params))
	if err := flattenComixParams(&pairs, "", reflect.ValueOf(map[string]any(params))); err != nil {
		return "", err
	}
	if len(pairs) > 0 {
		path += "?" + strings.Join(pairs, "&")
	}

	return path, nil
}

func flattenComixParams(pairs *[]string, prefix string, value reflect.Value) error {
	if !value.IsValid() || value.Kind() == reflect.Interface && value.IsNil() {
		return nil
	}
	for value.Kind() == reflect.Interface || value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return nil
		}
		value = value.Elem()
	}

	switch value.Kind() {
	case reflect.Map:
		keys := value.MapKeys()
		sort.Slice(keys, func(i, j int) bool {
			return fmt.Sprint(keys[i].Interface()) < fmt.Sprint(keys[j].Interface())
		})
		for _, key := range keys {
			name := fmt.Sprint(key.Interface())
			if name == comixTokenParameter {
				continue
			}
			nested := name
			if prefix != "" {
				nested = prefix + "[" + name + "]"
			}
			if err := flattenComixParams(pairs, nested, value.MapIndex(key)); err != nil {
				return err
			}
		}
	case reflect.Array, reflect.Slice:
		if value.Kind() == reflect.Slice && value.IsNil() {
			return nil
		}
		for i := range value.Len() {
			if err := flattenComixParams(pairs, prefix+"["+strconv.Itoa(i)+"]", value.Index(i)); err != nil {
				return err
			}
		}
	case reflect.String:
		*pairs = append(*pairs, prefix+"="+value.String())
	case reflect.Bool:
		*pairs = append(*pairs, prefix+"="+strconv.FormatBool(value.Bool()))
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		*pairs = append(*pairs, prefix+"="+strconv.FormatInt(value.Int(), 10))
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		*pairs = append(*pairs, prefix+"="+strconv.FormatUint(value.Uint(), 10))
	case reflect.Float32, reflect.Float64:
		*pairs = append(*pairs, prefix+"="+strconv.FormatFloat(value.Float(), 'g', -1, value.Type().Bits()))
	default:
		return fmt.Errorf("canonicalizing Comix parameter %q: unsupported type %s", prefix, value.Type())
	}

	return nil
}

func isComixProtectedPath(path string) bool {
	if path == "/manga" || strings.HasPrefix(path, "/manga/") {
		return true
	}
	if after, ok := strings.CutPrefix(path, "/chapters/"); ok {
		chapterID := after
		return chapterID != "" && !strings.Contains(chapterID, "/")
	}
	if !strings.HasPrefix(path, "/groups/") || !strings.HasSuffix(path, "/chapters") {
		return false
	}

	groupID := strings.TrimSuffix(strings.TrimPrefix(path, "/groups/"), "/chapters")
	if groupID == "" {
		return false
	}
	for _, character := range groupID {
		if character < '0' || character > '9' {
			return false
		}
	}

	return true
}

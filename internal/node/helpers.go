package node

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strconv"
	"time"
)

func canonicalizeJSON(data interface{}) ([]byte, error) {
	buffer := &bytes.Buffer{}
	if err := encodeCanonical(buffer, data); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func encodeCanonical(w *bytes.Buffer, v interface{}) error {
	switch val := v.(type) {
	case map[string]interface{}:
		w.WriteByte('{')
		keys := make([]string, 0, len(val))
		for k := range val {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for i, k := range keys {
			if i > 0 {
				w.WriteByte(',')
			}
			keyBytes, _ := json.Marshal(k)
			w.Write(keyBytes)
			w.WriteByte(':')
			if err := encodeCanonical(w, val[k]); err != nil {
				return err
			}
		}
		w.WriteByte('}')
	case []interface{}:
		w.WriteByte('[')
		for i, item := range val {
			if i > 0 {
				w.WriteByte(',')
			}
			if err := encodeCanonical(w, item); err != nil {
				return err
			}
		}
		w.WriteByte(']')
	case string:
		b, _ := json.Marshal(val)
		w.Write(b)
	case float64:
		w.WriteString(strconv.FormatFloat(val, 'f', -1, 64))
	case bool:
		if val {
			w.WriteString("true")
		} else {
			w.WriteString("false")
		}
	case nil:
		w.WriteString("null")
	default:
		b, err := json.Marshal(val)
		if err != nil {
			return err
		}
		w.Write(b)
	}
	return nil
}

func generarUUID() string {
	return hex.EncodeToString([]byte(time.Now().String()))[:16]
}

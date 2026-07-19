package session

import (
	"bytes"
	"errors"
	"os"
)

const (
	chunkSize = 256 * 1024
	maxScan   = 8 << 20 // これ以上遡っても見つからなければ諦める
)

// ErrNoAssistantText はテキスト応答が1つも見つからなかったことを示す。
var ErrNoAssistantText = errors.New("no assistant text response in session")

// LastAssistantText はセッション JSONL を末尾からチャンク単位で読み、
// 最後の（sidechain でない）テキスト付き assistant 応答を返す。
func LastAssistantText(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	st, err := f.Stat()
	if err != nil {
		return "", err
	}
	size := st.Size()

	var carry []byte // 直前のウィンドウ先頭の切れた行（このウィンドウ末尾に続く）
	off := size
	for off > 0 && size-off < maxScan {
		readStart := off - chunkSize
		if readStart < 0 {
			readStart = 0
		}
		buf := make([]byte, off-readStart)
		if _, err := f.ReadAt(buf, readStart); err != nil {
			return "", err
		}
		data := append(buf, carry...)
		lines := bytes.Split(data, []byte{'\n'})
		first := 0
		if readStart > 0 {
			// 先頭行はウィンドウ境界で切れている可能性があるため次周に持ち越す
			carry = append([]byte(nil), lines[0]...)
			first = 1
		} else {
			carry = nil
		}
		for i := len(lines) - 1; i >= first; i-- {
			if txt, ok := assistantText(lines[i]); ok {
				return txt, nil
			}
		}
		off = readStart
	}
	return "", ErrNoAssistantText
}

// tailLines はファイル末尾 n バイトを行に分割して返す（先頭の欠け行は捨てる）。
func tailLines(path string, n int64) ([][]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	st, err := f.Stat()
	if err != nil {
		return nil, err
	}
	start := st.Size() - n
	partial := start > 0
	if start < 0 {
		start = 0
	}
	buf := make([]byte, st.Size()-start)
	if _, err := f.ReadAt(buf, start); err != nil {
		return nil, err
	}
	lines := bytes.Split(buf, []byte{'\n'})
	if partial && len(lines) > 0 {
		lines = lines[1:]
	}
	return lines, nil
}

// headLines はファイル先頭 n バイトを行に分割して返す（末尾の欠け行は捨てる）。
func headLines(path string, n int64) ([][]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	st, err := f.Stat()
	if err != nil {
		return nil, err
	}
	partial := st.Size() > n
	if n > st.Size() {
		n = st.Size()
	}
	buf := make([]byte, n)
	if _, err := f.ReadAt(buf, 0); err != nil {
		return nil, err
	}
	lines := bytes.Split(buf, []byte{'\n'})
	if partial && len(lines) > 0 {
		lines = lines[:len(lines)-1]
	}
	return lines, nil
}

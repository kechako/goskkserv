package dict

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"sync"

	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/transform"
)

type Dictionary struct {
	table map[string]*entry
	mu    sync.RWMutex
}

var magicCommentRegex = regexp.MustCompile(`-\*-.*[ \t]coding:[ \t]*([^ \t;]+?)[ \t;].*-\*-`)

func (d *Dictionary) Add(name string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.table == nil {
		d.table = make(map[string]*entry)
	}

	file, err := os.Open(name)
	if err != nil {
		return fmt.Errorf("failed to open dictionary file %s: %w", name, err)
	}
	defer file.Close()

	r := bufio.NewReader(file)
	first, err := r.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read dictionary %s: %w", name, err)
	}

	enc := "euc-jp"
	matches := magicCommentRegex.FindStringSubmatch(first)
	if len(matches) > 1 {
		enc = matches[1]
	}
	r, err = wrapEncDecoder(r, enc)
	if err != nil {
		return err
	}

	for {
		line, err := r.ReadString('\n')
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return fmt.Errorf("failed to read dictionary %s: %w", name, err)
		}
		if line[0] == ';' {
			continue
		}

		i := strings.IndexByte(line, ' ')
		if i < 0 {
			continue
		}
		key := line[:i]
		candidates := strings.Split(line[i+1:len(line)-1], "/")

		entry := d.table[key]
		if entry == nil {
			entry = newEntry()
			d.table[key] = entry
		}

		for _, candidate := range candidates {
			if candidate == "" {
				continue
			}

			var text string
			var annotation string
			ai := strings.IndexByte(candidate, ';')
			if ai < 0 {
				text = candidate
			} else {
				text = candidate[:ai]
				annotation = candidate[ai+1:]
			}
			entry.add(text, annotation)
		}
	}

	return nil
}

func wrapEncDecoder(r io.Reader, enc string) (*bufio.Reader, error) {
	var br *bufio.Reader
	switch enc {
	case "euc-jp", "euc-jis-2004":
		br = bufio.NewReader(transform.NewReader(r, japanese.EUCJP.NewDecoder()))
	case "sjis":
		br = bufio.NewReader(transform.NewReader(r, japanese.ShiftJIS.NewDecoder()))
	case "utf-8":
		br = bufio.NewReader(r)
	default:
		return nil, fmt.Errorf("unsupported encoding: %s", enc)
	}

	return br, nil
}

func (d *Dictionary) Search(key string) []Candidate {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if d.table == nil {
		return nil
	}

	entry, ok := d.table[key]
	if !ok {
		return nil
	}

	return entry.Candidates()
}

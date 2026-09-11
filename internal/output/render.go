package output

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
)

const (
	tableColumnPadding    = 3
	keyValueColumnPadding = 2
)

func Table(w io.Writer, headers []string, rows [][]string) error {
	tw := tabwriter.NewWriter(w, 0, 0, tableColumnPadding, ' ', 0)
	if _, err := fmt.Fprintln(tw, strings.Join(sanitizeAll(headers), "\t")); err != nil {
		return err
	}
	for _, row := range rows {
		if _, err := fmt.Fprintln(tw, strings.Join(sanitizeAll(row), "\t")); err != nil {
			return err
		}
	}
	return tw.Flush()
}

func KeyValue(w io.Writer, pairs [][2]string) error {
	tw := tabwriter.NewWriter(w, 0, 0, keyValueColumnPadding, ' ', 0)
	for _, p := range pairs {
		if _, err := fmt.Fprintf(tw, "%s:\t%s\n", SanitizeText(p[0]), SanitizeText(p[1])); err != nil {
			return err
		}
	}
	return tw.Flush()
}

func sanitizeAll(ss []string) []string {
	out := make([]string, len(ss))
	for i, s := range ss {
		out[i] = SanitizeText(s)
	}
	return out
}

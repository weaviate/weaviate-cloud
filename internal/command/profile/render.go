package profile

import (
	"io"
	"strconv"

	"github.com/weaviate/weaviate-cloud/internal/output"
)

const keyActive = "active"

func renderProfileList(w io.Writer, d profileListData) error {
	rows := make([][]string, len(d.Profiles))
	for i, p := range d.Profiles {
		rows[i] = []string{p.Name, strconv.FormatBool(p.Active)}
	}
	return output.Table(w, []string{"NAME", "ACTIVE"}, rows)
}

func renderProfileShow(w io.Writer, d profileShowData) error {
	return output.KeyValue(w, [][2]string{
		{"name", d.Name},
		{keyActive, strconv.FormatBool(d.Active)},
		{"endpoint_configured", strconv.FormatBool(d.EndpointConfigured)},
		{"auth_configured", strconv.FormatBool(d.AuthConfigured)},
	})
}

func renderProfileUse(w io.Writer, d profileUseData) error {
	return output.KeyValue(w, [][2]string{
		{keyActive, d.Active},
	})
}

func renderProfileCreate(w io.Writer, d profileCreateData) error {
	return output.KeyValue(w, [][2]string{
		{"name", d.Name},
		{keyActive, strconv.FormatBool(d.Active)},
	})
}

func renderProfileDelete(w io.Writer, d profileDeleteData) error {
	return output.KeyValue(w, [][2]string{
		{"deleted", d.Deleted},
		{keyActive, d.Active},
	})
}

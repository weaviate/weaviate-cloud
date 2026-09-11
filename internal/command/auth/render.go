package auth

import (
	"io"
	"strconv"

	"github.com/weaviate/weaviate-cloud/internal/api"
	"github.com/weaviate/weaviate-cloud/internal/output"
)

func renderWhoAmI(w io.Writer, info *api.WhoAmI) error {
	return output.KeyValue(w, [][2]string{
		{"user_id", info.UserID},
		{"email", info.Email},
		{"org_id", info.OrgID},
	})
}

func renderLogin(w io.Writer, r loginResult) error {
	pairs := [][2]string{{"signed_in", strconv.FormatBool(r.SignedIn)}}
	if r.Email != "" {
		pairs = append(pairs, [2]string{"email", r.Email})
	}
	return output.KeyValue(w, pairs)
}

func renderLogout(w io.Writer, r logoutResult) error {
	return output.KeyValue(w, [][2]string{{"signed_out", strconv.FormatBool(r.SignedOut)}})
}

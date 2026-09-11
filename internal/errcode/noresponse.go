package errcode

import "fmt"

const (
	DetailFailureStage    = "failure_stage"
	FailureStageTransport = "transport"
)

const (
	PeerAuthServer = "the authentication server"
	PeerCloudAPI   = "the Weaviate Cloud API"
)

// NoResponse reports that a request produced no HTTP response at all, so nothing was
// accepted and nothing was rejected. Both hostnames are named whichever leg failed,
// because a reader who has to open an allowlist needs the pair, not the one that
// happened to be tried first.
func NoResponse(peer string, cause error) *Error {
	return &Error{
		Code: CodeInternalError,
		Message: fmt.Sprintf(
			"no response was received from %s: the request did not complete in transport, so it was "+
				"neither accepted nor rejected. That is what a network policy, proxy or agent sandbox "+
				"blocking outbound traffic looks like, and also what a dropped or unreachable "+
				"connection looks like. wcloud needs outbound access to auth.weaviate.cloud and "+
				"api-cloud.weaviate.cloud; where an organisation manages that allowlist centrally, "+
				"only an administrator can change it, because a central allowlist replaces the local "+
				"one rather than merging with it. Run 'wcloud guide' for the per-environment "+
				"configuration paths",
			peer,
		),
		Details: map[string]any{DetailFailureStage: FailureStageTransport},
		Cause:   cause,
	}
}

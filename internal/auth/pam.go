package auth

/*
#cgo LDFLAGS: -lpam
#include <stdlib.h>
#include <string.h>
#include <security/pam_appl.h>

static int nimbus_conv(int num_msg, const struct pam_message **msg,
                        struct pam_response **resp, void *appdata_ptr) {
	struct pam_response *reply = calloc(num_msg, sizeof(struct pam_response));
	if (reply == NULL) {
		return PAM_BUF_ERR;
	}
	const char *password = (const char *)appdata_ptr;
	int i;
	for (i = 0; i < num_msg; i++) {
		reply[i].resp_retcode = 0;
		reply[i].resp = NULL;
		if (msg[i]->msg_style == PAM_PROMPT_ECHO_OFF || msg[i]->msg_style == PAM_PROMPT_ECHO_ON) {
			reply[i].resp = strdup(password);
		}
	}
	*resp = reply;
	return PAM_SUCCESS;
}

// nimbus_start hides PAM's raw function-pointer struct field from cgo, which
// cannot express it directly on the Go side.
static int nimbus_start(const char *service, const char *user, const char *password, pam_handle_t **pamh) {
	struct pam_conv conv;
	conv.conv = nimbus_conv;
	conv.appdata_ptr = (void *)password;
	return pam_start(service, user, &conv, pamh);
}
*/
import "C"

import (
	"errors"
	"fmt"
	"unsafe"
)

// ErrAuthFailed is returned for any PAM authentication or account failure.
// Deliberately generic so callers never leak which check failed.
var ErrAuthFailed = errors.New("authentication failed")

// DefaultService is the PAM service name nimbusfs authenticates against.
// Operators can add /etc/pam.d/nimbusfs to customize the stack; otherwise
// it falls back to whatever the system provides for that name (often "login"-like defaults).
const DefaultService = "nimbusfs"

// Authenticate validates a username/password pair against the system PAM stack.
// No password is stored or cached by nimbusfs; PAM owns the credential check.
func Authenticate(service, username, password string) error {
	if service == "" {
		service = DefaultService
	}

	cService := C.CString(service)
	defer C.free(unsafe.Pointer(cService))
	cUsername := C.CString(username)
	defer C.free(unsafe.Pointer(cUsername))
	cPassword := C.CString(password)
	defer C.free(unsafe.Pointer(cPassword))

	var pamh *C.pam_handle_t
	rc := C.nimbus_start(cService, cUsername, cPassword, &pamh)
	if rc != C.PAM_SUCCESS {
		return fmt.Errorf("pam_start: %d", int(rc))
	}
	defer C.pam_end(pamh, rc)

	rc = C.pam_authenticate(pamh, 0)
	if rc != C.PAM_SUCCESS {
		return ErrAuthFailed
	}

	rc = C.pam_acct_mgmt(pamh, 0)
	if rc != C.PAM_SUCCESS {
		return ErrAuthFailed
	}

	return nil
}

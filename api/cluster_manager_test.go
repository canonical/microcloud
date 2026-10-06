package api

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateLXDURL(t *testing.T) {
	cases := []struct {
		name          string
		value         string
		expectedError string
	}{
		{
			name:  "HTTP hostname",
			value: "http://example.com:8443",
		},
		{
			name:  "HTTPS hostname",
			value: "https://example.com:8443",
		},
		{
			name:  "IPv4 host",
			value: "https://192.168.1.10:8443",
		},
		{
			name:  "IPv6 host",
			value: "https://[2001:db8::1]:8443",
		},
		{
			name:  "URL with path",
			value: "https://example.com/ui",
		},
		{
			name:          "Hostname without scheme",
			value:         "example.com/ui",
			expectedError: `URL "example.com/ui" must be an absolute URL starting with "http://" or "https://"`,
		},
		{
			name:          "IPv4 host without scheme",
			value:         "192.168.1.10:8443",
			expectedError: `URL "192.168.1.10:8443" must be an absolute URL starting with "http://" or "https://"`,
		},
		{
			name:          "Relative path",
			value:         "/path",
			expectedError: `URL "/path" must be an absolute URL starting with "http://" or "https://"`,
		},
		{
			name:          "Incomplete HTTP prefix",
			value:         "http:/path",
			expectedError: `URL "http:/path" must be an absolute URL starting with "http://" or "https://"`,
		},
		{
			name:          "Port without hostname",
			value:         "https://:8443",
			expectedError: `URL "https://:8443" is missing a host`,
		},
		{
			name:          "Disallowed scheme",
			value:         "ftp://example.com",
			expectedError: `URL "ftp://example.com" must be an absolute URL starting with "http://" or "https://"`,
		},
		{
			name:          "Missing host",
			value:         "https:///path",
			expectedError: `URL "https:///path" is missing a host`,
		},
		{
			name:          "Invalid port",
			value:         "https://example.com:bogus",
			expectedError: `parse "https://example.com:bogus": invalid port ":bogus" after host`,
		},
		{
			name:          "Invalid URL escape",
			value:         "https://example.com/%zz",
			expectedError: `parse "https://example.com/%zz": invalid URL escape "%zz"`,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			err := validateLXDURL(testCase.value)
			if testCase.expectedError != "" {
				require.EqualError(t, err, testCase.expectedError)
				return
			}

			require.NoError(t, err)
		})
	}
}

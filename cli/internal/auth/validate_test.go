package auth

import "testing"

func TestValidateIssuerURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{name: "https public host", url: "https://idp.example.com", wantErr: false},
		{name: "https with path", url: "https://idp.example.com/realms/myrealm", wantErr: false},
		{name: "http localhost", url: "http://localhost:8080", wantErr: false},
		{name: "http 127.0.0.1", url: "http://127.0.0.1:8080", wantErr: false},
		{name: "http ::1", url: "http://[::1]:8080", wantErr: false},
		{name: "http non-localhost blocked", url: "http://idp.example.com", wantErr: true},
		{name: "http internal IP blocked", url: "http://192.168.1.1", wantErr: true},
		{name: "file scheme blocked", url: "file:///etc/passwd", wantErr: true},
		{name: "ftp scheme blocked", url: "ftp://evil.com", wantErr: true},
		{name: "https link-local IP blocked", url: "https://169.254.169.254", wantErr: true},
		{name: "https private 10.x blocked", url: "https://10.0.0.1", wantErr: true},
		{name: "https private 172.16.x blocked", url: "https://172.16.0.1", wantErr: true},
		{name: "https private 192.168.x blocked", url: "https://192.168.1.1", wantErr: true},
		{name: "https loopback blocked", url: "https://127.0.0.1", wantErr: true},
		{name: "https unspecified blocked", url: "https://0.0.0.0", wantErr: true},
		{name: "empty string", url: "", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateIssuerURL(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateIssuerURL(%q) error = %v, wantErr %v", tt.url, err, tt.wantErr)
			}
		})
	}
}

func TestValidateEndpointOrigin(t *testing.T) {
	tests := []struct {
		name      string
		endpoint  string
		issuerURL string
		wantErr   bool
	}{
		{name: "matching origin", endpoint: "https://idp.example.com/token", issuerURL: "https://idp.example.com/realms/myrealm", wantErr: false},
		{name: "matching with port", endpoint: "https://idp.example.com:8443/token", issuerURL: "https://idp.example.com:8443/realms/myrealm", wantErr: false},
		{name: "different host", endpoint: "https://evil.com/token", issuerURL: "https://idp.example.com", wantErr: true},
		{name: "different scheme", endpoint: "http://idp.example.com/token", issuerURL: "https://idp.example.com", wantErr: true},
		{name: "different port", endpoint: "https://idp.example.com:9999/token", issuerURL: "https://idp.example.com:8443", wantErr: true},
		{name: "http localhost matching", endpoint: "http://127.0.0.1:8080/token", issuerURL: "http://127.0.0.1:8080", wantErr: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateEndpointOrigin(tt.endpoint, tt.issuerURL)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateEndpointOrigin(%q, %q) error = %v, wantErr %v", tt.endpoint, tt.issuerURL, err, tt.wantErr)
			}
		})
	}
}

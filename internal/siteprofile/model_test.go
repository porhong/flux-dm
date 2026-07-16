package siteprofile

import "testing"

func TestMatchPrefersMostSpecificHost(t *testing.T) {
	records := []Record{{Profile: Profile{ID: "wild", HostPattern: "*.example.com"}}, {Profile: Profile{ID: "exact", HostPattern: "files.example.com"}}}
	match := Match("https://files.example.com/a", records)
	if match == nil || match.Profile.ID != "exact" {
		t.Fatalf("match=%#v", match)
	}
}
func TestValidateRejectsHeaderInjectionAndProxyCredentials(t *testing.T) {
	profile := Profile{Name: "x", HostPattern: "example.com", AuthType: AuthNone}
	if ValidateProfile(profile, SecretPayload{Headers: map[string]string{"X-Test": "yes\r\nInjected: no"}}) == nil {
		t.Fatal("accepted injected header")
	}
	profile.ProxyURL = "http://user:pass@proxy.test"
	if ValidateProfile(profile, SecretPayload{}) == nil {
		t.Fatal("accepted credentials in proxy URL")
	}
}

package application

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/fluxdm/fluxdm/internal/download"
	"github.com/fluxdm/fluxdm/internal/secrets"
	"github.com/fluxdm/fluxdm/internal/siteprofile"
)

type SaveSiteProfileInput struct {
	ID            string               `json:"id"`
	Name          string               `json:"name"`
	HostPattern   string               `json:"hostPattern"`
	AuthType      siteprofile.AuthType `json:"authType"`
	Username      string               `json:"username"`
	Password      string               `json:"password"`
	BearerToken   string               `json:"bearerToken"`
	Cookies       string               `json:"cookies"`
	Headers       map[string]string    `json:"headers"`
	ProxyURL      string               `json:"proxyUrl"`
	ProxyUsername string               `json:"proxyUsername"`
	ProxyPassword string               `json:"proxyPassword"`
}
type RequestProfileResolver interface {
	Resolve(context.Context, string, string, string) (string, download.RequestOptions, error)
	SaveDownloadCookies(context.Context, string, string) error
	ClearDownloadSecrets(context.Context, string) error
}
type SiteProfileService struct {
	repository siteprofile.Repository
	protector  secrets.Protector
}

func NewSiteProfileService(repository siteprofile.Repository, protector secrets.Protector) *SiteProfileService {
	return &SiteProfileService{repository: repository, protector: protector}
}
func (s *SiteProfileService) List(ctx context.Context) ([]siteprofile.Profile, error) {
	records, err := s.repository.List(ctx)
	if err != nil {
		return nil, NewError(ErrInternal, "Could not list site profiles.", err)
	}
	result := make([]siteprofile.Profile, 0, len(records))
	for _, record := range records {
		payload, err := s.unprotect(record.EncryptedSecrets)
		if err != nil {
			return nil, NewError(ErrInternal, "Could not unlock site profile.", err)
		}
		profile := record.Profile
		profile.HasCredentials = payload.Username != "" || payload.Password != "" || payload.BearerToken != "" || payload.ProxyUsername != "" || payload.ProxyPassword != ""
		profile.HasCookies = payload.Cookies != ""
		for name := range payload.Headers {
			profile.HeaderNames = append(profile.HeaderNames, name)
		}
		sort.Strings(profile.HeaderNames)
		result = append(result, profile)
	}
	return result, nil
}
func (s *SiteProfileService) Save(ctx context.Context, input SaveSiteProfileInput) (siteprofile.Profile, error) {
	id := strings.TrimSpace(input.ID)
	if id == "" {
		id = newID()
	} else if _, err := validateID(id); err != nil {
		return siteprofile.Profile{}, NewError(ErrInvalidInput, "Invalid profile identifier.", err)
	}
	profile := siteprofile.Profile{ID: id, Name: strings.TrimSpace(input.Name), HostPattern: strings.ToLower(strings.TrimSpace(input.HostPattern)), AuthType: input.AuthType, ProxyURL: strings.TrimSpace(input.ProxyURL), CreatedAt: time.Now().UTC()}
	payload := siteprofile.SecretPayload{Username: input.Username, Password: input.Password, BearerToken: input.BearerToken, Cookies: input.Cookies, Headers: input.Headers, ProxyUsername: input.ProxyUsername, ProxyPassword: input.ProxyPassword}
	if err := siteprofile.ValidateProfile(profile, payload); err != nil {
		return siteprofile.Profile{}, NewError(ErrInvalidInput, "Check the site profile settings.", err)
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return siteprofile.Profile{}, NewError(ErrInternal, "Could not encode protected settings.", err)
	}
	ciphertext, err := s.protector.Protect(encoded)
	clear(encoded)
	if err != nil {
		return siteprofile.Profile{}, NewError(ErrInternal, "Could not protect site profile secrets.", err)
	}
	if err := s.repository.Save(ctx, siteprofile.Record{Profile: profile, EncryptedSecrets: ciphertext}); err != nil {
		return siteprofile.Profile{}, NewError(ErrInternal, "Could not save site profile.", err)
	}
	profile.HasCredentials = payload.Username != "" || payload.Password != "" || payload.BearerToken != "" || payload.ProxyUsername != "" || payload.ProxyPassword != ""
	profile.HasCookies = payload.Cookies != ""
	for name := range payload.Headers {
		profile.HeaderNames = append(profile.HeaderNames, name)
	}
	sort.Strings(profile.HeaderNames)
	return profile, nil
}
func (s *SiteProfileService) Delete(ctx context.Context, id string) error {
	id, err := validateID(id)
	if err != nil {
		return NewError(ErrInvalidInput, "Invalid profile identifier.", err)
	}
	if err := s.repository.Delete(ctx, id); err != nil {
		return repositoryError("delete site profile", err)
	}
	return nil
}
func (s *SiteProfileService) ClearSecrets(ctx context.Context, id string) error {
	record, err := s.repository.Get(ctx, id)
	if err != nil {
		return repositoryError("clear site profile", err)
	}
	record.Profile.AuthType = siteprofile.AuthNone
	encoded := []byte(`{}`)
	ciphertext, err := s.protector.Protect(encoded)
	if err != nil {
		return NewError(ErrInternal, "Could not protect cleared profile.", err)
	}
	record.EncryptedSecrets = ciphertext
	if err := s.repository.Save(ctx, record); err != nil {
		return NewError(ErrInternal, "Could not clear site profile secrets.", err)
	}
	return nil
}
func (s *SiteProfileService) Resolve(ctx context.Context, rawURL, profileID, downloadID string) (string, download.RequestOptions, error) {
	var record *siteprofile.Record
	if profileID != "" {
		value, err := s.repository.Get(ctx, profileID)
		if err != nil {
			return "", download.RequestOptions{}, err
		}
		record = &value
	} else {
		records, err := s.repository.List(ctx)
		if err != nil {
			return "", download.RequestOptions{}, err
		}
		record = siteprofile.Match(rawURL, records)
	}
	options := download.RequestOptions{Headers: map[string]string{}}
	resolvedID := ""
	if record != nil {
		payload, err := s.unprotect(record.EncryptedSecrets)
		if err != nil {
			return "", options, err
		}
		resolvedID = record.Profile.ID
		for name, value := range payload.Headers {
			options.Headers[http.CanonicalHeaderKey(name)] = value
		}
		switch record.Profile.AuthType {
		case siteprofile.AuthBasic:
			options.Headers["Authorization"] = "Basic " + base64.StdEncoding.EncodeToString([]byte(payload.Username+":"+payload.Password))
		case siteprofile.AuthBearer:
			options.Headers["Authorization"] = "Bearer " + payload.BearerToken
		}
		if payload.Cookies != "" {
			options.Headers["Cookie"] = payload.Cookies
		}
		options.ProxyURL = record.Profile.ProxyURL
		options.ProxyUsername = payload.ProxyUsername
		options.ProxyPassword = payload.ProxyPassword
	}
	if downloadID != "" {
		ciphertext, err := s.repository.GetDownloadSecret(ctx, downloadID)
		if err != nil {
			return "", options, err
		}
		if len(ciphertext) > 0 {
			cookies, err := s.protector.Unprotect(ciphertext)
			if err != nil {
				return "", options, err
			}
			if len(cookies) > 0 {
				options.Headers["Cookie"] = string(cookies)
			}
			clear(cookies)
		}
	}
	return resolvedID, options, nil
}
func (s *SiteProfileService) SaveDownloadCookies(ctx context.Context, id, cookies string) error {
	if cookies == "" {
		return nil
	}
	if len(cookies) > 32768 || strings.ContainsAny(cookies, "\r\n") {
		return NewError(ErrInvalidInput, "Browser cookies were invalid.", nil)
	}
	ciphertext, err := s.protector.Protect([]byte(cookies))
	if err != nil {
		return err
	}
	return s.repository.SaveDownloadSecret(ctx, id, ciphertext)
}
func (s *SiteProfileService) ClearDownloadSecrets(ctx context.Context, id string) error {
	return s.repository.DeleteDownloadSecret(ctx, id)
}
func (s *SiteProfileService) unprotect(ciphertext []byte) (siteprofile.SecretPayload, error) {
	plaintext, err := s.protector.Unprotect(ciphertext)
	if err != nil {
		return siteprofile.SecretPayload{}, err
	}
	defer clear(plaintext)
	var payload siteprofile.SecretPayload
	if len(plaintext) > 0 {
		err = json.Unmarshal(plaintext, &payload)
	}
	return payload, err
}

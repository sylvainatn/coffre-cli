// Package api est un client minimal pour l'API Coffre.
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Client struct {
	BaseURL   string
	Token     string
	UserAgent string
	HTTP      *http.Client
}

func New(baseURL, token, userAgent string) *Client {
	return &Client{
		BaseURL:   strings.TrimRight(baseURL, "/"),
		Token:     token,
		UserAgent: userAgent,
		HTTP:      &http.Client{Timeout: 30 * time.Second},
	}
}

// Error est une erreur renvoyée par l'API ({ "error": "...", "message": "..." }).
type Error struct {
	Status  int
	Code    string `json:"error"`
	Message string `json:"message"`
}

func (e *Error) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return fmt.Sprintf("erreur HTTP %d", e.Status)
}

func (c *Client) do(ctx context.Context, method, path string, body, out any) error {
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, reader)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.UserAgent)
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}

	res, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("impossible de joindre %s : %w", c.BaseURL, err)
	}
	defer res.Body.Close()
	data, err := io.ReadAll(io.LimitReader(res.Body, 32<<20))
	if err != nil {
		return err
	}

	if res.StatusCode >= 400 {
		apiErr := &Error{Status: res.StatusCode}
		_ = json.Unmarshal(data, apiErr)
		if res.StatusCode == http.StatusUnauthorized && apiErr.Code != "invalid_credentials" {
			apiErr.Message = "Session expirée ou invalide. Lancez `coffre login`."
		}
		return apiErr
	}
	if out != nil {
		if err := json.Unmarshal(data, out); err != nil {
			return fmt.Errorf("réponse inattendue de l'API : %w", err)
		}
	}
	return nil
}

/* --------------------------------- Types --------------------------------- */

type User struct {
	ID    string  `json:"id"`
	Email string  `json:"email"`
	Name  *string `json:"name"`
}

type Environment struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	SecretCount int    `json:"secretCount"`
}

type Project struct {
	ID           string        `json:"id"`
	Name         string        `json:"name"`
	Role         string        `json:"role"`
	Environments []Environment `json:"environments"`
}

type Secret struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type DeviceAuth struct {
	DeviceCode              string `json:"deviceCode"`
	UserCode                string `json:"userCode"`
	VerificationURL         string `json:"verificationUrl"`
	VerificationURLComplete string `json:"verificationUrlComplete"`
	ExpiresIn               int    `json:"expiresIn"`
	Interval                int    `json:"interval"`
}

/* ------------------------------- Endpoints ------------------------------- */

func (c *Client) Me(ctx context.Context) (*User, error) {
	var out struct {
		User User `json:"user"`
	}
	if err := c.do(ctx, "GET", "/v1/me", nil, &out); err != nil {
		return nil, err
	}
	return &out.User, nil
}

func (c *Client) StartDeviceAuth(ctx context.Context, clientName string) (*DeviceAuth, error) {
	var out DeviceAuth
	err := c.do(ctx, "POST", "/v1/auth/device", map[string]string{"clientName": clientName}, &out)
	return &out, err
}

// PollDeviceToken renvoie (token, user, nil) une fois approuvé,
// ou une *Error de code "authorization_pending" tant que l'utilisateur n'a pas validé.
func (c *Client) PollDeviceToken(ctx context.Context, deviceCode string) (string, *User, error) {
	var out struct {
		Token string `json:"token"`
		User  User   `json:"user"`
	}
	if err := c.do(ctx, "POST", "/v1/auth/device/token", map[string]string{"deviceCode": deviceCode}, &out); err != nil {
		return "", nil, err
	}
	return out.Token, &out.User, nil
}

func (c *Client) Logout(ctx context.Context) error {
	return c.do(ctx, "POST", "/v1/auth/logout", map[string]string{}, nil)
}

// ProjectSummary : un projet de la liste (sans ses environnements).
type ProjectSummary struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	Role             string `json:"role"`
	EnvironmentCount int    `json:"environmentCount"`
}

func (c *Client) ListProjects(ctx context.Context) ([]ProjectSummary, error) {
	var out struct {
		Projects []ProjectSummary `json:"projects"`
	}
	if err := c.do(ctx, "GET", "/v1/projects", nil, &out); err != nil {
		return nil, err
	}
	return out.Projects, nil
}

// CreateProject crée un projet (avec ses environnements par défaut) et renvoie son ID.
func (c *Client) CreateProject(ctx context.Context, name string) (string, error) {
	var out struct {
		Project struct {
			ID string `json:"id"`
		} `json:"project"`
	}
	if err := c.do(ctx, "POST", "/v1/projects", map[string]string{"name": name}, &out); err != nil {
		return "", err
	}
	return out.Project.ID, nil
}

func (c *Client) GetProject(ctx context.Context, projectID string) (*Project, error) {
	var out struct {
		Project Project `json:"project"`
	}
	if err := c.do(ctx, "GET", "/v1/projects/"+url.PathEscape(projectID), nil, &out); err != nil {
		return nil, err
	}
	return &out.Project, nil
}

func secretsPath(projectID, env string) string {
	return "/v1/projects/" + url.PathEscape(projectID) + "/environments/" + url.PathEscape(env) + "/secrets"
}

func (c *Client) GetSecrets(ctx context.Context, projectID, env string) ([]Secret, error) {
	var out struct {
		Secrets []Secret `json:"secrets"`
	}
	if err := c.do(ctx, "GET", secretsPath(projectID, env), nil, &out); err != nil {
		return nil, err
	}
	return out.Secrets, nil
}

func (c *Client) SetSecrets(ctx context.Context, projectID, env string, secrets []Secret) error {
	return c.do(ctx, "PUT", secretsPath(projectID, env), map[string]any{"secrets": secrets}, nil)
}

// UpsertResult : en mode create_only, ce qui a été créé et ce qui existait déjà.
type UpsertResult struct {
	Count   int      `json:"count"`
	Created []string `json:"created"`
	Skipped []string `json:"skipped"`
}

// CreateMissingSecrets crée uniquement les secrets absents de Coffre, sans jamais écraser
// une valeur existante (la vérification est faite par l'API, dans la même transaction).
func (c *Client) CreateMissingSecrets(ctx context.Context, projectID, env string, secrets []Secret) (*UpsertResult, error) {
	var out UpsertResult
	body := map[string]any{"secrets": secrets, "mode": "create_only"}
	if err := c.do(ctx, "PUT", secretsPath(projectID, env), body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) DeleteSecret(ctx context.Context, projectID, env, key string) error {
	return c.do(ctx, "DELETE", secretsPath(projectID, env)+"/"+url.PathEscape(key), nil, nil)
}

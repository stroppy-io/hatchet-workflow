package agent

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const (
	EnvAgentToken           = "STROPPY_AGENT_TOKEN"
	DefaultAgentTokenTTL    = 30 * 24 * time.Hour
	defaultAgentTokenIssuer = "stroppy-cloud-agent"
	defaultAgentAudience    = "stroppy-agent"
)

type TokenClaims struct {
	TenantID  string `json:"tenant_id"`
	RunID     string `json:"run_id"`
	MachineID string `json:"machine_id"`
	TaskQueue string `json:"task_queue"`
	jwt.RegisteredClaims
}

type TokenService struct {
	secret []byte
	issuer string
	ttl    time.Duration
	now    func() time.Time
}

func NewTokenService(signingSecret string) (*TokenService, error) {
	signingSecret = strings.TrimSpace(signingSecret)
	if signingSecret == "" {
		return nil, errors.New("agent token signing secret is not configured")
	}
	return &TokenService{
		secret: deriveAgentTokenSecret(signingSecret),
		issuer: defaultAgentTokenIssuer,
		ttl:    DefaultAgentTokenTTL,
		now:    time.Now,
	}, nil
}

func (s *TokenService) IssueAgentToken(tenantID, runID, machineID, taskQueue string) (string, error) {
	if s == nil {
		return "", errors.New("agent token service is not configured")
	}
	tenantID = strings.TrimSpace(tenantID)
	runID = strings.TrimSpace(runID)
	machineID = strings.TrimSpace(machineID)
	taskQueue = strings.TrimSpace(taskQueue)
	if tenantID == "" || runID == "" || machineID == "" || taskQueue == "" {
		return "", errors.New("agent token requires tenant id, run id, machine id and task queue")
	}

	now := s.now().UTC()
	claims := TokenClaims{
		TenantID:  tenantID,
		RunID:     runID,
		MachineID: machineID,
		TaskQueue: taskQueue,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   machineID,
			Issuer:    s.issuer,
			Audience:  jwt.ClaimStrings{defaultAgentAudience},
			ID:        uuid.NewString(),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.ttl)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.secret)
}

func (s *TokenService) VerifyAgentToken(token string) (*TokenClaims, error) {
	if s == nil {
		return nil, errors.New("agent token service is not configured")
	}
	claims := &TokenClaims{}
	parsed, err := jwt.ParseWithClaims(strings.TrimSpace(token), claims, s.keyFunc,
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(s.issuer),
		jwt.WithAudience(defaultAgentAudience),
		jwt.WithExpirationRequired(),
	)
	if err != nil || !parsed.Valid {
		return nil, errors.New("invalid agent token")
	}
	if claims.TenantID == "" || claims.RunID == "" || claims.MachineID == "" || claims.TaskQueue == "" {
		return nil, errors.New("agent token has incomplete claims")
	}
	if claims.Subject != "" && claims.Subject != claims.MachineID {
		return nil, fmt.Errorf("agent token subject %q does not match machine %q", claims.Subject, claims.MachineID)
	}
	return claims, nil
}

func (s *TokenService) keyFunc(token *jwt.Token) (interface{}, error) {
	if token.Method.Alg() != jwt.SigningMethodHS256.Alg() {
		return nil, fmt.Errorf("unexpected agent token signing method %s", token.Method.Alg())
	}
	return s.secret, nil
}

func TaskQueue(machineID string) string {
	machineID = strings.TrimSpace(machineID)
	if machineID == "" {
		return "stroppy-agent"
	}
	return "stroppy-agent-" + machineID
}

func NewTaskQueue(machineID string) string {
	return TaskQueueWithNonce(machineID, uuid.NewString())
}

func TaskQueueWithNonce(machineID, nonce string) string {
	machineID = strings.TrimSpace(machineID)
	nonce = strings.ReplaceAll(strings.TrimSpace(nonce), "-", "")
	if machineID == "" {
		machineID = "agent"
	}
	if nonce == "" {
		return TaskQueue(machineID)
	}
	return "stroppy-agent-" + machineID + "-" + nonce
}

func BearerToken(header string) string {
	header = strings.TrimSpace(header)
	if len(header) <= 7 || !strings.EqualFold(header[:7], "bearer ") {
		return ""
	}
	return strings.TrimSpace(header[7:])
}

func deriveAgentTokenSecret(signingSecret string) []byte {
	sum := sha256.Sum256([]byte("stroppy-agent-token:" + signingSecret))
	return sum[:]
}

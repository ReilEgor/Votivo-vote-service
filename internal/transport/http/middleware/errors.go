// Package middleware holds the Gin middleware of the vote HTTP API.
package middleware

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/ReilEgor/Votivo-vote-service/internal/domain/model"
	"github.com/ReilEgor/Votivo-vote-service/internal/transport/http/dto"
)

// ErrUnauthorized is returned when the gateway did not pass the user.
var ErrUnauthorized = errors.New("unauthorized: user is not identified")

// BodyError reports a request body that cannot be used.
type BodyError struct{ Err error }

// Error implements error.
func (e BodyError) Error() string { return "invalid request body: " + e.Err.Error() }

// Unwrap returns the cause.
func (e BodyError) Unwrap() error { return e.Err }

// ValidationError reports every field that failed validation.
type ValidationError struct{ Errs []error }

// Error implements error.
func (e ValidationError) Error() string {
	msgs := make([]string, 0, len(e.Errs))
	for _, err := range e.Errs {
		msgs = append(msgs, err.Error())
	}
	return strings.Join(msgs, "; ")
}

// Unwrap returns the individual errors.
func (e ValidationError) Unwrap() []error { return e.Errs }

type rule struct {
	target error
	status int
	level  slog.Level
}

// catalog maps domain errors to responses. The text of the error is the text of the response.
var catalog = []rule{
	{ErrUnauthorized, http.StatusUnauthorized, slog.LevelWarn},
	{model.ErrPollNotFound, http.StatusNotFound, slog.LevelWarn},
	{model.ErrVoteNotFound, http.StatusNotFound, slog.LevelWarn},
	{model.ErrPollNotActive, http.StatusConflict, slog.LevelWarn},
	{model.ErrAlreadyVoted, http.StatusConflict, slog.LevelWarn},
	{model.ErrVoteNotChangeable, http.StatusForbidden, slog.LevelWarn},
	{model.ErrUnknownQuestion, http.StatusUnprocessableEntity, slog.LevelWarn},
	{model.ErrUnknownOption, http.StatusUnprocessableEntity, slog.LevelWarn},
	{model.ErrDuplicateAnswer, http.StatusUnprocessableEntity, slog.LevelWarn},
	{model.ErrQuestionNotAnswered, http.StatusUnprocessableEntity, slog.LevelWarn},
	{model.ErrTooManyOptions, http.StatusUnprocessableEntity, slog.LevelWarn},
	{model.ErrPollServiceUnavailable, http.StatusServiceUnavailable, slog.LevelError},
}

// fallbackText is the 500 text of each route.
var fallbackText = map[string]string{
	"POST /api/v1/polls/:poll_id/vote":   "failed to cast vote",
	"PUT /api/v1/polls/:poll_id/vote":    "failed to change vote",
	"DELETE /api/v1/polls/:poll_id/vote": "failed to revoke vote",
	"GET /api/v1/polls/:poll_id/vote":    "failed to get vote status",
}

const defaultFallbackText = "internal error"

// Fail attaches err to the request; HandleErrors turns it into the response.
func Fail(c *gin.Context, err error) {
	c.Errors = append(c.Errors, &gin.Error{Err: err, Type: gin.ErrorTypePrivate})
}

// HandleErrors is the single place that turns an error into a response and a log record.
// Handlers only attach the error with middleware.Fail.
func HandleErrors() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		if len(c.Errors) == 0 || c.Writer.Written() {
			return
		}
		err := c.Errors.Last().Err
		statusCode, body, level := resolve(c, err)

		slog.Log(c.Request.Context(), level, "request failed",
			slog.String("route", c.Request.Method+" "+c.FullPath()),
			slog.Int("status", statusCode),
			slog.String("error", err.Error()),
		)
		c.AbortWithStatusJSON(statusCode, body)
	}
}

func resolve(c *gin.Context, err error) (int, any, slog.Level) {
	var validation ValidationError
	if errors.As(err, &validation) {
		texts := make([]string, 0, len(validation.Errs))
		for _, e := range validation.Errs {
			texts = append(texts, e.Error())
		}
		return http.StatusBadRequest, dto.ErrorsResponse{Errors: texts}, slog.LevelWarn
	}
	var bodyErr BodyError
	if errors.As(err, &bodyErr) {
		return http.StatusBadRequest, dto.ErrorResponse{Error: bodyErr.Error()}, slog.LevelWarn
	}
	for _, r := range catalog {
		if errors.Is(err, r.target) {
			return r.status, dto.ErrorResponse{Error: r.target.Error()}, r.level
		}
	}
	text, ok := fallbackText[c.Request.Method+" "+c.FullPath()]
	if !ok {
		text = defaultFallbackText
	}
	return http.StatusInternalServerError, dto.ErrorResponse{Error: text}, slog.LevelError
}

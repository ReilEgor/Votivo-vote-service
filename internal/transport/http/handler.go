// Package http exposes the vote use cases as a REST API.
package http

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/ReilEgor/Votivo-vote-service/internal/domain/model"
	"github.com/ReilEgor/Votivo-vote-service/internal/domain/usecase"
	"github.com/ReilEgor/Votivo-vote-service/internal/transport/http/dto"
	"github.com/ReilEgor/Votivo-vote-service/internal/transport/http/middleware"
)

const (
	timeoutCast   = 5 * time.Second
	timeoutChange = 5 * time.Second
	timeoutRevoke = 5 * time.Second
	timeoutStatus = 3 * time.Second
)

// Handler serves the vote API.
type Handler struct {
	votes usecase.VoteUseCase
}

// NewHandler returns a Handler.
func NewHandler(votes usecase.VoteUseCase) *Handler {
	return &Handler{votes: votes}
}

// Cast accepts the first vote of the user: POST /api/v1/polls/:poll_id/vote.
func (h *Handler) Cast(c *gin.Context) {
	pollID, answers, ok := parseVote(c)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), timeoutCast)
	defer cancel()

	if err := h.votes.Cast(ctx, middleware.UserIDFrom(c), pollID, answers); err != nil {
		middleware.Fail(c, err)
		return
	}
	c.JSON(http.StatusCreated, dto.MessageResponse{Message: "vote accepted"})
}

// Change replaces the vote of the user: PUT /api/v1/polls/:poll_id/vote.
func (h *Handler) Change(c *gin.Context) {
	pollID, answers, ok := parseVote(c)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), timeoutChange)
	defer cancel()

	if err := h.votes.Change(ctx, middleware.UserIDFrom(c), pollID, answers); err != nil {
		middleware.Fail(c, err)
		return
	}
	c.JSON(http.StatusOK, dto.MessageResponse{Message: "vote changed"})
}

// Revoke withdraws the vote of the user: DELETE /api/v1/polls/:poll_id/vote.
func (h *Handler) Revoke(c *gin.Context) {
	pollID, err := parsePollID(c)
	if err != nil {
		middleware.Fail(c, middleware.ValidationError{Errs: []error{err}})
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), timeoutRevoke)
	defer cancel()

	if err = h.votes.Revoke(ctx, middleware.UserIDFrom(c), pollID); err != nil {
		middleware.Fail(c, err)
		return
	}
	c.JSON(http.StatusOK, dto.MessageResponse{Message: "vote revoked"})
}

// Status tells whether the user has voted: GET /api/v1/polls/:poll_id/vote.
func (h *Handler) Status(c *gin.Context) {
	pollID, err := parsePollID(c)
	if err != nil {
		middleware.Fail(c, middleware.ValidationError{Errs: []error{err}})
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), timeoutStatus)
	defer cancel()

	status, err := h.votes.Status(ctx, middleware.UserIDFrom(c), pollID)
	if err != nil {
		middleware.Fail(c, err)
		return
	}
	c.JSON(http.StatusOK, dto.NewVoteStatusResponse(status))
}

// parseVote reads the poll ID and the body and checks their format. On failure it attaches the error
// to the request and returns false.
func parseVote(c *gin.Context) (int64, []model.Answer, bool) {
	pollID, idErr := parsePollID(c)

	var req dto.VoteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, middleware.BodyError{Err: err})
		return 0, nil, false
	}

	errs := validateAnswers(req.Answers)
	if idErr != nil {
		errs = append([]error{idErr}, errs...)
	}
	if len(errs) > 0 {
		middleware.Fail(c, middleware.ValidationError{Errs: errs})
		return 0, nil, false
	}
	return pollID, req.ToAnswers(), true
}

var errInvalidPollID = errors.New("poll_id must be a positive integer")

func parsePollID(c *gin.Context) (int64, error) {
	id, err := strconv.ParseInt(c.Param("poll_id"), 10, 64)
	if err != nil || id <= 0 {
		return 0, errInvalidPollID
	}
	return id, nil
}

// validateAnswers checks the format of the answers against the limits of a poll. Whether the IDs exist
// is the business of the use case, which knows the poll.
func validateAnswers(answers []dto.AnswerDTO) []error {
	var errs []error
	if len(answers) == 0 {
		return append(errs, errors.New("answers must not be empty"))
	}
	if len(answers) > model.MaxQuestions {
		errs = append(errs, fmt.Errorf("answers must not exceed %d questions", model.MaxQuestions))
	}
	for i, a := range answers {
		if a.QuestionID <= 0 {
			errs = append(errs, fmt.Errorf("answers[%d].question_id must be a positive integer", i))
		}
		if len(a.OptionIDs) == 0 || len(a.OptionIDs) > model.MaxOptionsPerQuestion {
			errs = append(errs, fmt.Errorf("answers[%d].option_ids must hold 1 to %d options",
				i, model.MaxOptionsPerQuestion))
		}
	}
	return errs
}

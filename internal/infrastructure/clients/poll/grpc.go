// Package poll gets the voting rules of a poll from Poll Service.
package poll

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/ReilEgor/Votivo-vote-service/internal/domain/model"
	"github.com/ReilEgor/Votivo-vote-service/pkg/ctxlog"
	pb "github.com/ReilEgor/Votivo-vote-service/pkg/grpc/pollv1"
)

const metadataRequestID = "x-request-id"

// GRPCClient calls PollService and translates its answers into domain values and errors.
type GRPCClient struct {
	client pb.PollServiceClient
}

// NewGRPCClient returns a GRPCClient on conn.
func NewGRPCClient(conn grpc.ClientConnInterface) *GRPCClient {
	return &GRPCClient{client: pb.NewPollServiceClient(conn)}
}

// GetPoll asks Poll Service for the poll. The deadline of ctx bounds the call.
func (c *GRPCClient) GetPoll(ctx context.Context, id int64) (model.Poll, error) {
	const op = "GRPCClient.GetPoll"

	if reqID := ctxlog.RequestID(ctx); reqID != "" {
		ctx = metadata.AppendToOutgoingContext(ctx, metadataRequestID, reqID)
	}

	resp, err := c.client.GetPoll(ctx, &pb.GetPollRequest{Id: id})
	if err != nil {
		return model.Poll{}, fmt.Errorf("%s: %w", op, translate(err))
	}

	questions := make([]model.Question, 0, len(resp.GetQuestions()))
	for _, q := range resp.GetQuestions() {
		questions = append(
			questions,
			model.Question{ID: q.GetId(), Multiple: q.GetMultiple(), OptionIDs: q.GetOptionIds()},
		)
	}
	return model.Poll{
		ID:          resp.GetId(),
		Status:      toStatus(resp.GetStatus()),
		Anonymous:   resp.GetAnonymous(),
		AllowChange: resp.GetAllowVoteChange(),
		Questions:   questions,
	}, nil
}

// toStatus maps the protobuf status; an unknown one is treated as a draft, which never accepts votes.
func toStatus(s pb.PollStatus) model.PollStatus {
	switch s {
	case pb.PollStatus_POLL_STATUS_ACTIVE:
		return model.PollStatusActive
	case pb.PollStatus_POLL_STATUS_CLOSED:
		return model.PollStatusClosed
	case pb.PollStatus_POLL_STATUS_ARCHIVED:
		return model.PollStatusArchived
	case pb.PollStatus_POLL_STATUS_BLOCKED:
		return model.PollStatusBlocked
	case pb.PollStatus_POLL_STATUS_DRAFT, pb.PollStatus_POLL_STATUS_UNSPECIFIED:
		return model.PollStatusDraft
	default:
		return model.PollStatusDraft
	}
}

// domainErrors maps the gRPC codes Poll Service uses for expected outcomes to domain errors.
var domainErrors = map[codes.Code]error{
	codes.NotFound:          model.ErrPollNotFound,
	codes.Unavailable:       model.ErrPollServiceUnavailable,
	codes.ResourceExhausted: model.ErrPollServiceUnavailable,
	codes.DeadlineExceeded:  model.ErrPollServiceUnavailable,
}

// translate maps a gRPC status to a domain error; any other failure stays an internal error.
func translate(err error) error {
	if domainErr, ok := domainErrors[status.Code(err)]; ok {
		return fmt.Errorf("%w: %s", domainErr, status.Code(err))
	}
	return fmt.Errorf("poll call failed: %w", err)
}

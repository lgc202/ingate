package mysql

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	changebiz "github.com/lgc202/ingate/internal/assistant/biz/change"
	"github.com/lgc202/ingate/internal/assistant/data/mysql/db"
)

func proposedChangeFromDB(item db.AssistantProposedChange) (changebiz.ProposedChange, error) {
	if uuid.Validate(item.ID) != nil || uuid.Validate(item.ConversationID) != nil ||
		uuid.Validate(item.ExecutionID) != nil || item.CallID == "" ||
		uuid.Validate(item.InterruptID) != nil ||
		item.Summary == "" || strings.TrimSpace(item.Summary) != item.Summary ||
		len(item.Summary) > 1024 || item.CreatedAt.IsZero() {
		return changebiz.ProposedChange{}, fmt.Errorf("invalid stored proposed change %q", item.ID)
	}
	proposal, err := proposalFromJSON(item.ProposalJson)
	if err != nil {
		return changebiz.ProposedChange{}, err
	}
	if proposedChangeKindToDB(proposal.Kind) != item.Kind {
		return changebiz.ProposedChange{}, fmt.Errorf(
			"proposed change %s kind does not match its configuration",
			item.ID,
		)
	}
	state, err := proposedChangeStateFromDB(item.State)
	if err != nil {
		return changebiz.ProposedChange{}, err
	}
	result := changebiz.ProposedChange{
		ID:             item.ID,
		ConversationID: item.ConversationID,
		ExecutionID:    item.ExecutionID,
		InterruptID:    item.InterruptID,
		State:          state,
		Summary:        item.Summary,
		Proposal:       proposal,
		ResourceID:     item.ResourceID,
		ErrorCode:      changebiz.FailureCode(item.ErrorCode),
		CreatedAt:      item.CreatedAt,
	}
	if item.DecidedAt.Valid {
		result.DecidedAt = &item.DecidedAt.Time
	}
	if item.FinishedAt.Valid {
		result.FinishedAt = &item.FinishedAt.Time
	}
	if err := validateStoredProposedChange(item, result); err != nil {
		return changebiz.ProposedChange{}, err
	}
	return result, nil
}

func proposalFromJSON(value json.RawMessage) (changebiz.Proposal, error) {
	var proposal changebiz.Proposal
	decoder := json.NewDecoder(bytes.NewReader(value))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&proposal); err != nil {
		return changebiz.Proposal{}, fmt.Errorf("unmarshal proposed change configuration: %w", err)
	}
	if err := proposal.Validate(); err != nil {
		return changebiz.Proposal{}, fmt.Errorf("validate proposed change configuration: %w", err)
	}
	return proposal, nil
}

func proposedChangeKindToDB(kind changebiz.Kind) uint8 {
	switch kind {
	case changebiz.KindCreateGateway:
		return proposedChangeKindCreateGateway
	case changebiz.KindCreateService:
		return proposedChangeKindCreateService
	default:
		return 0
	}
}

func proposedChangeStateToDB(state changebiz.State) uint8 {
	switch state {
	case changebiz.StatePendingReview:
		return proposedChangeStatePendingReview
	case changebiz.StateExecuting:
		return proposedChangeStateExecuting
	case changebiz.StateSucceeded:
		return proposedChangeStateSucceeded
	case changebiz.StateRejected:
		return proposedChangeStateRejected
	case changebiz.StateFailed:
		return proposedChangeStateFailed
	case changebiz.StateOutcomeUnknown:
		return proposedChangeStateOutcomeUnknown
	default:
		return 0
	}
}

func proposedChangeStateFromDB(state uint8) (changebiz.State, error) {
	switch state {
	case proposedChangeStatePendingReview:
		return changebiz.StatePendingReview, nil
	case proposedChangeStateExecuting:
		return changebiz.StateExecuting, nil
	case proposedChangeStateSucceeded:
		return changebiz.StateSucceeded, nil
	case proposedChangeStateRejected:
		return changebiz.StateRejected, nil
	case proposedChangeStateFailed:
		return changebiz.StateFailed, nil
	case proposedChangeStateOutcomeUnknown:
		return changebiz.StateOutcomeUnknown, nil
	default:
		return "", fmt.Errorf("invalid proposed change state %d", state)
	}
}

func validateCompletion(
	state changebiz.State,
	resource changebiz.CreatedResource,
	errorCode changebiz.FailureCode,
) error {
	switch state {
	case changebiz.StateSucceeded:
		if uuid.Validate(resource.ID) != nil || errorCode != "" {
			return errors.New("successful proposed change contains an invalid result")
		}
	case changebiz.StateFailed:
		if resource.ID != "" || errorCode != changebiz.FailureAdminRejected {
			return errors.New("failed proposed change contains an invalid result")
		}
	case changebiz.StateOutcomeUnknown:
		if resource.ID != "" || errorCode != changebiz.FailureOutcomeUnknown {
			return errors.New("uncertain proposed change contains an invalid result")
		}
	default:
		return fmt.Errorf("cannot complete proposed change with state %q", state)
	}
	return nil
}

func validateStoredProposedChange(
	stored db.AssistantProposedChange,
	result changebiz.ProposedChange,
) error {
	if stored.DecidedAt.Valid && stored.DecidedAt.Time.Before(stored.CreatedAt) ||
		stored.FinishedAt.Valid && stored.FinishedAt.Time.Before(stored.CreatedAt) ||
		stored.DecidedAt.Valid && stored.FinishedAt.Valid &&
			stored.FinishedAt.Time.Before(stored.DecidedAt.Time) {
		return fmt.Errorf("proposed change %s contains invalid timestamps", stored.ID)
	}
	switch result.State {
	case changebiz.StatePendingReview:
		if stored.DecidedAt.Valid || stored.FinishedAt.Valid ||
			result.ResourceID != "" || result.ErrorCode != "" {
			return fmt.Errorf("pending proposed change %s contains execution state", stored.ID)
		}
	case changebiz.StateExecuting:
		if !stored.DecidedAt.Valid || stored.FinishedAt.Valid ||
			result.ResourceID != "" || result.ErrorCode != "" {
			return fmt.Errorf("executing proposed change %s is incomplete", stored.ID)
		}
	case changebiz.StateSucceeded:
		if !stored.DecidedAt.Valid || !stored.FinishedAt.Valid ||
			uuid.Validate(result.ResourceID) != nil ||
			result.ErrorCode != "" {
			return fmt.Errorf("succeeded proposed change %s is inconsistent", stored.ID)
		}
	case changebiz.StateRejected:
		if !stored.DecidedAt.Valid || !stored.FinishedAt.Valid ||
			result.ResourceID != "" || result.ErrorCode != "" {
			return fmt.Errorf("rejected proposed change %s is inconsistent", stored.ID)
		}
	case changebiz.StateFailed:
		if !stored.DecidedAt.Valid || !stored.FinishedAt.Valid || result.ResourceID != "" ||
			result.ErrorCode != changebiz.FailureAdminRejected {
			return fmt.Errorf("failed proposed change %s is inconsistent", stored.ID)
		}
	case changebiz.StateOutcomeUnknown:
		if !stored.DecidedAt.Valid || !stored.FinishedAt.Valid || result.ResourceID != "" ||
			result.ErrorCode != changebiz.FailureOutcomeUnknown {
			return fmt.Errorf("uncertain proposed change %s is inconsistent", stored.ID)
		}
	}
	return nil
}

func changeNotFound(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return changebiz.ErrNotFound
	}
	return err
}

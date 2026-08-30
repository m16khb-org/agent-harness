package remotecmd

import (
	artifacttemplate "agent-harness/internal/domain/artifacttemplate"
	issueopsremote "agent-harness/internal/domain/issueopsremote"
	policydomain "agent-harness/internal/domain/policy"
	port "agent-harness/internal/port"
	"context"
	"crypto/sha256"
	"errors"
	"flag"
	"fmt"
	"strings"
	"time"

	issueopscontract "agent-harness/internal/contract/issueops"
)

func runRemoteCreateIssue(ctx context.Context, args []string, deps Deps) error {
	fs := flag.NewFlagSet("issueops remote create-issue", flag.ContinueOnError)
	id := fs.String("id", "", "IssueOps id")
	title := fs.String("title", "", "issue title")
	body := fs.String("body", "", "issue body (markdown)")
	bodyFile := fs.String("body-file", "", "issue body markdown file")
	template := fs.String("template", "", "template kind")
	providerOverride := fs.String("provider", "", "remote provider override: github or gitlab")
	scoreFile := fs.String("score-file", "", "IssueOps remote score result JSON")
	confirm := fs.Bool("confirm", false, "execute creation; without this, dry-run preview only")
	var labels repeatedFlag
	var assignees repeatedFlag
	var fields repeatedFlag
	fs.Var(&labels, "label", "label to apply (repeatable)")
	fs.Var(&assignees, "assignee", "assignee username (repeatable)")
	fs.Var(&fields, "field", "template field key=value (canonical or documented alias; repeatable)")
	jsonOut := fs.Bool("json", false, "print JSON")
	if help, err := parseFlags(fs, args); help || err != nil {
		return err
	}
	labels, assignees = normalizeRemoteCreateMetadata(labels, assignees)
	record, err := remoteDeps.ReadIssueOps(remoteDeps.IssueOpsStateRoot(), *id)
	if err != nil {
		return deps.printErrorResult(*jsonOut, err)
	}
	if strings.TrimSpace(*title) == "" {
		return deps.printErrorResult(*jsonOut, fmt.Errorf("issue title is required"))
	}
	providerName := firstNonEmptyMain(*providerOverride, remoteDeps.ResolveRecordProvider(record))
	if providerName == "" {
		// 최초 이슈를 만드는 명령이 이미 존재하는 issue_url을 요구하면 bootstrap
		// 순환이다. record가 침묵하면 저장소 remote에 물어본다(#300). 판별이
		// 모호하면 추측하지 않고 --provider 복구 명령을 안내한다.
		inferred, inferErr := inferProviderFromRepoRemotes(record.Repo)
		if inferErr != nil {
			return deps.printErrorResult(*jsonOut, inferErr)
		}
		providerName = inferred
	}
	prov, err := Resolve(providerName)
	if err != nil {
		return deps.printErrorResult(*jsonOut, err)
	}
	finalBody, err := resolveTemplateBody(resolveTemplateBodyRequest{
		Kind:      artifacttemplate.IssueOpsArtifactIssue,
		Template:  *template,
		Provider:  providerName,
		Title:     *title,
		Body:      *body,
		BodyFile:  *bodyFile,
		Fields:    fields,
		ScoreFile: *scoreFile,
	})
	if err != nil {
		return deps.printErrorResult(*jsonOut, err)
	}
	if err := rejectSecretLikeRemoteCreateInputs("issue create", *title, finalBody, labels, assignees); err != nil {
		return deps.printErrorResult(*jsonOut, err)
	}
	if err := validateConfirmRemoteCreate(*confirm, labels, assignees); err != nil {
		return deps.printErrorResult(*jsonOut, err)
	}
	requestBody := finalBody
	projectAuthority := ""
	if *confirm {
		startedAt := time.Now().UTC().Format(time.RFC3339Nano)
		operationID := ""
		marker := ""
		if record.IssueCreateIntent != nil {
			operationID = record.IssueCreateIntent.OperationID
			marker = record.IssueCreateIntent.Marker
		} else {
			operationSeed := sha256.Sum256([]byte(record.ID + "\n" + startedAt + "\n" + *title))
			operationID = fmt.Sprintf("%x", operationSeed[:16])
			marker = "<!-- agent-harness:issue-create:" + operationID + " -->"
		}
		requestBody = strings.TrimSpace(finalBody)
		if requestBody == "" {
			requestBody = marker
		} else {
			requestBody += "\n\n" + marker
		}
		bodyDigest := sha256.Sum256([]byte(requestBody))
		var authorityErr error
		projectAuthority, authorityErr = remoteDeps.ResolveProviderProjectAuthority(record.Repo, providerName)
		if authorityErr != nil {
			return deps.printErrorResult(*jsonOut, authorityErr)
		}
		if _, err := remoteDeps.BeginIssueCreateIntent(remoteDeps.IssueOpsStateRoot(), record.ID, issueopscontract.IssueOpsIssueCreateIntentRequest{
			OperationID:      operationID,
			Provider:         providerName,
			ProjectAuthority: projectAuthority,
			Title:            *title,
			BodySHA256:       fmt.Sprintf("%x", bodyDigest[:]),
			Labels:           append([]string(nil), labels...),
			Assignees:        append([]string(nil), assignees...),
			StartedAt:        startedAt,
		}); err != nil {
			return deps.printErrorResult(*jsonOut, err)
		}
	}
	request := port.IssueProviderCreateIssueRequest{
		Repo:       record.Repo,
		ProjectKey: projectAuthority,
		Title:      *title,
		Body:       requestBody,
		Labels:     labels,
		Assignees:  assignees,
		Confirm:    *confirm,
	}
	var result port.IssueProviderCreateIssueResult
	if *confirm {
		result, err = remoteDeps.CreateRemoteIssueContext(ctx, request, prov)
	} else {
		result, err = remoteDeps.CreateRemoteIssue(request, prov)
	}
	if err != nil {
		if *confirm {
			status := issueopscontract.IssueCreateIntentInvokedUnknown
			if createErr, ok := errors.AsType[*port.IssueProviderCreateError](err); ok && !createErr.Invoked {
				status = issueopscontract.IssueCreateIntentNotInvoked
			} else if strings.TrimSpace(result.URL) != "" {
				status = issueopscontract.IssueCreateIntentURLObserved
			}
			if _, stateErr := remoteDeps.RecordIssueCreateOutcome(remoteDeps.IssueOpsStateRoot(), record.ID, issueopscontract.IssueOpsIssueCreateOutcome{
				Status:       status,
				CanonicalURL: result.URL,
				Failure:      durableIssueCreateFailure(err),
				ObservedAt:   time.Now().UTC().Format(time.RFC3339Nano),
			}); stateErr != nil {
				err = errors.Join(err, stateErr)
			}
		}
		return deps.printErrorResult(*jsonOut, err)
	}
	result.Provider = providerName
	result.Labels = issueopsremote.CleanValues(labels)
	result.Assignees = issueopsremote.CleanValues(assignees)
	// Mirror create-child's verification gate: once an issue is really created,
	// confirm the live issue carries the requested labels/assignees before the
	// command reports success. Without --confirm this is a dry-run preview only.
	if *confirm && strings.TrimSpace(result.URL) != "" {
		if err := deps.verifyLive(ctx, issueopscontract.IssueOpsRemoteArtifactVerificationRequest{
			Provider:  providerName,
			Kind:      "issue",
			URL:       result.URL,
			Labels:    labels,
			Assignees: assignees,
		}); err != nil {
			if _, stateErr := remoteDeps.RecordIssueCreateOutcome(remoteDeps.IssueOpsStateRoot(), record.ID, issueopscontract.IssueOpsIssueCreateOutcome{
				Status:       issueopscontract.IssueCreateIntentVerificationFailed,
				CanonicalURL: result.URL,
				Failure:      durableIssueCreateFailure(err),
				ObservedAt:   time.Now().UTC().Format(time.RFC3339Nano),
			}); stateErr != nil {
				err = errors.Join(err, stateErr)
			}
			return deps.printErrorResult(*jsonOut, err)
		}
		if _, err := remoteDeps.CompleteIssueCreateIntent(remoteDeps.IssueOpsStateRoot(), record.ID, result.URL, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
			if _, stateErr := remoteDeps.RecordIssueCreateOutcome(remoteDeps.IssueOpsStateRoot(), record.ID, issueopscontract.IssueOpsIssueCreateOutcome{
				Status:       issueopscontract.IssueCreateIntentReceiptFailed,
				CanonicalURL: result.URL,
				Failure:      durableIssueCreateFailure(err),
				ObservedAt:   time.Now().UTC().Format(time.RFC3339Nano),
			}); stateErr != nil {
				err = errors.Join(err, stateErr)
			}
			return deps.printErrorResult(*jsonOut, err)
		}
	}
	if *jsonOut {
		return deps.printJSON(result)
	}
	if result.URL != "" {
		fmt.Printf("created: %s\n", result.URL)
	} else {
		fmt.Println(result.Preview)
	}
	return nil
}

func rejectSecretLikeRemoteCreateInputs(kind, title, body string, labels, assignees []string) error {
	values := []struct {
		field string
		value string
	}{
		{field: "title", value: title},
		{field: "body", value: body},
	}
	for _, label := range labels {
		values = append(values, struct {
			field string
			value string
		}{field: "label", value: label})
	}
	for _, assignee := range assignees {
		values = append(values, struct {
			field string
			value string
		}{field: "assignee", value: assignee})
	}
	for _, candidate := range values {
		if policydomain.RedactFreeform(candidate.value) != candidate.value {
			return fmt.Errorf("%s %s contains secret-like content", kind, candidate.field)
		}
	}
	return nil
}

func runRemoteReconcileIssue(ctx context.Context, args []string, deps Deps) error {
	fs := flag.NewFlagSet("issueops remote reconcile-issue", flag.ContinueOnError)
	id := fs.String("id", "", "IssueOps id")
	confirm := fs.Bool("confirm", false, "adopt the unique live verified issue")
	jsonOut := fs.Bool("json", false, "print JSON")
	if help, err := parseFlags(fs, args); help || err != nil {
		return err
	}
	record, err := remoteDeps.ReadIssueOps(remoteDeps.IssueOpsStateRoot(), *id)
	if err != nil {
		return deps.printErrorResult(*jsonOut, err)
	}
	if record.IssueCreateIntent == nil {
		return deps.printErrorResult(*jsonOut, fmt.Errorf("no issue create intent to reconcile"))
	}
	if record.IssueCreateIntent.Status == issueopscontract.IssueCreateIntentCompleted {
		return deps.printErrorResult(*jsonOut, fmt.Errorf("issue create intent is already completed"))
	}
	prov, err := Resolve(record.IssueCreateIntent.Provider)
	if err != nil {
		return deps.printErrorResult(*jsonOut, err)
	}
	reconciler, ok := prov.(port.IssueProviderIssueCreateReconciler)
	if !ok {
		return deps.printErrorResult(*jsonOut, fmt.Errorf("provider %s does not support issue create reconciliation", record.IssueCreateIntent.Provider))
	}
	searchResult, err := reconciler.FindIssueCreateCandidates(ctx, port.IssueProviderFindIssueCreateCandidatesRequest{
		Repo:             record.Repo,
		ProjectAuthority: record.IssueCreateIntent.ProjectAuthority,
		Marker:           record.IssueCreateIntent.Marker,
	})
	if err != nil {
		return deps.printErrorResult(*jsonOut, err)
	}
	if searchResult.Truncated {
		return deps.printErrorResult(*jsonOut, fmt.Errorf("issue create reconciliation search was truncated; uniqueness is indeterminate"))
	}
	candidates := searchResult.Candidates
	if len(candidates) != 1 {
		return deps.printErrorResult(*jsonOut, fmt.Errorf("issue create reconciliation found %d live candidates; exactly one is required", len(candidates)))
	}
	candidate := candidates[0]
	candidateAuthority := issueopsremote.ProjectKey(candidate.URL, record.IssueCreateIntent.Provider, "issue")
	if candidateAuthority == "" || candidateAuthority != record.IssueCreateIntent.ProjectAuthority {
		err := fmt.Errorf(
			"issue create reconciliation candidate authority %q does not match sealed authority %q",
			candidateAuthority,
			record.IssueCreateIntent.ProjectAuthority,
		)
		if *confirm {
			_, stateErr := remoteDeps.RecordIssueCreateOutcome(remoteDeps.IssueOpsStateRoot(), record.ID, issueopscontract.IssueOpsIssueCreateOutcome{
				Status:       issueopscontract.IssueCreateIntentVerificationFailed,
				CanonicalURL: candidate.URL,
				Failure:      durableIssueCreateFailure(err),
				ObservedAt:   time.Now().UTC().Format(time.RFC3339Nano),
			})
			err = errors.Join(err, stateErr)
		}
		return deps.printErrorResult(*jsonOut, err)
	}
	bodyDigest := sha256.Sum256([]byte(candidate.Body))
	if candidate.Title != record.IssueCreateIntent.Title ||
		fmt.Sprintf("%x", bodyDigest[:]) != record.IssueCreateIntent.BodySHA256 {
		err := fmt.Errorf("issue create reconciliation candidate does not match sealed title and body digest")
		if *confirm {
			_, stateErr := remoteDeps.RecordIssueCreateOutcome(remoteDeps.IssueOpsStateRoot(), record.ID, issueopscontract.IssueOpsIssueCreateOutcome{
				Status:       issueopscontract.IssueCreateIntentVerificationFailed,
				CanonicalURL: candidate.URL,
				Failure:      durableIssueCreateFailure(err),
				ObservedAt:   time.Now().UTC().Format(time.RFC3339Nano),
			})
			err = errors.Join(err, stateErr)
		}
		return deps.printErrorResult(*jsonOut, err)
	}
	if !*confirm {
		result := issueopscontract.IssueOpsIssueCreateReconcileResult{
			OK:                true,
			CandidateCount:    1,
			CandidateURL:      candidate.URL,
			WouldAdopt:        true,
			IssueURL:          record.IssueURL,
			IssueCreateIntent: record.IssueCreateIntent,
		}
		if *jsonOut {
			return deps.printJSON(result)
		}
		fmt.Printf("would adopt: %s\n", candidate.URL)
		return nil
	}
	if err := deps.verifyLive(ctx, issueopscontract.IssueOpsRemoteArtifactVerificationRequest{
		Provider:  record.IssueCreateIntent.Provider,
		Kind:      "issue",
		URL:       candidate.URL,
		Labels:    record.IssueCreateIntent.Labels,
		Assignees: record.IssueCreateIntent.Assignees,
	}); err != nil {
		_, stateErr := remoteDeps.RecordIssueCreateOutcome(remoteDeps.IssueOpsStateRoot(), record.ID, issueopscontract.IssueOpsIssueCreateOutcome{
			Status:       issueopscontract.IssueCreateIntentVerificationFailed,
			CanonicalURL: candidate.URL,
			Failure:      durableIssueCreateFailure(err),
			ObservedAt:   time.Now().UTC().Format(time.RFC3339Nano),
		})
		return deps.printErrorResult(*jsonOut, errors.Join(err, stateErr))
	}
	updated, err := remoteDeps.CompleteIssueCreateIntent(remoteDeps.IssueOpsStateRoot(), record.ID, candidate.URL, time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		return deps.printErrorResult(*jsonOut, err)
	}
	if *jsonOut {
		return deps.printJSON(issueopscontract.IssueOpsIssueCreateReconcileResult{
			OK:                true,
			CandidateCount:    1,
			CandidateURL:      candidate.URL,
			WouldAdopt:        false,
			IssueURL:          updated.IssueURL,
			IssueCreateIntent: updated.IssueCreateIntent,
		})
	}
	fmt.Printf("adopted: %s\n", candidate.URL)
	return nil
}

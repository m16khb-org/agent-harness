package lifecycle

import (
	"agent-harness/internal/adapter/lifecycle/model"
	lifecyclecontract "agent-harness/internal/contract/lifecycle"
	projectdoccontract "agent-harness/internal/contract/projectdoc"
	"agent-harness/internal/domain/projectdoc"
)

const ProjectDocsDir = projectdoc.ProjectDocsDir
const ProjectLifecycleSchemaVersion = model.ProjectLifecycleSchemaVersion
const projectLifecycleProfileFile = model.ProjectLifecycleProfileFile
const docUpkeepQueueFile = model.DocUpkeepQueueFile
const compactCapsuleFile = model.CompactCapsuleFile

type ProjectProfile = projectdoccontract.ProjectProfile
type ProjectFingerprint = lifecyclecontract.ProjectFingerprint
type ProjectLifecycleProfile = lifecyclecontract.ProjectLifecycleProfile
type ProjectLifecycleStatePlan = lifecyclecontract.ProjectLifecycleStatePlan

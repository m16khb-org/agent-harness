package lifecycle

import (
	"issueops/internal/adapter/lifecycle/model"
	lifecyclecontract "issueops/internal/contract/lifecycle"
	projectdoccontract "issueops/internal/contract/projectdoc"
	"issueops/internal/domain/projectdoc"
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

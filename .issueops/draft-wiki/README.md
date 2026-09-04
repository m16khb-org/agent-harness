# Draft Wiki

이 디렉토리는 issueops가 제안한 wiki 후보를 사용자가 검토하는 repo-local staging area다.

- `draft/`: 에이전트 노트 등에서 선별된 후보. 아직 승인되지 않았다.
- `approved/`: 사용자가 export를 승인한 후보.
- `rejected/`: 사용자가 거절한 후보.

주의: 이 디렉토리는 외부 wiki vault가 아니다. `promote --confirm`은 승인된 후보를 repo-local `exported/` 디렉토리로 이동하고 export log만 남긴다.

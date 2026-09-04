package installutil

import installcontract "issueops/internal/contract/install"

// 채택 계획은 계약이 소유한다. 어댑터는 같은 이름으로 재노출만 한다.
type ManagedCommandPathPlan = installcontract.ManagedCommandPathPlan

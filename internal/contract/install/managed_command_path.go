// 관리 대상 명령 파일 채택 계획이다. 채택을 수행하는 쪽은 파일시스템을 만지지만
// 계획을 읽고 결과로 옮기는 쪽은 그 구현을 알 필요가 없다.
package install

type ManagedCommandPathPlan struct {
	Path              string
	Target            string
	BackupPath        string
	AdoptionApproved  bool
	WouldAdopt        bool
	Adopted           bool
	Committed         bool
	RolledBack        bool
	RollbackAvailable bool
	BackupRetained    bool
}

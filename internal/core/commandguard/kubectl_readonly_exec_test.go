package commandguard

import (
	"strings"
	"testing"
)

func TestEvaluateGitOpsKubectlClassifiesExactReadOnlyExec(t *testing.T) {
	commands := []string{
		"kubectl --context bc-stgdev -n stg exec deploy/gateway -- getent hosts grpc-user",
		"kubectl exec --context=bc-stgdev --namespace=stg deploy/gateway -- nslookup grpc-user.stg.svc.cluster.local",
		"kubectl --namespace stg exec deploy/gateway --context bc-stgdev -- dig grpc-user",
		"kubectl --context bc-stgdev exec -n stg deploy/gateway -- dig +short grpc-user",
		"kubectl --context bc-stgdev -n stg exec deploy/gateway -- dig grpc-user A",
		"kubectl --context bc-stgdev -n stg exec deploy/gateway -- dig grpc-user AAAA",
		"kubectl --context bc-stgdev -n stg exec deploy/gateway -- dig +short grpc-user A",
		"kubectl --context bc-stgdev -n stg exec -c proxy deploy/gateway -- dig +short grpc-user AAAA",
		"kubectl --context bc-stgdev -n stg exec deploy/gateway -- cat /etc/resolv.conf",
		"kubectl --context bc-stgdev -n stg exec deploy/gateway -- curl -fsS http://localhost:4191/metrics",
		"kubectl --context bc-stgdev -n stg exec deploy/gateway -- curl -fsS http://127.0.0.1:4191/metrics",
	}
	for _, command := range commands {
		t.Run(command, func(t *testing.T) {
			got := EvaluateGitOpsKubectl("Bash", command)
			if got.Decision != "ask" || got.LiveAccess != KubectlLiveAccessReadOnlyExec || got.ExecScope.Context != "bc-stgdev" || got.ExecScope.Namespace != "stg" {
				t.Fatalf("classification = %+v", got)
			}
		})
	}
}

func TestEvaluateGitOpsKubectlRejectsUnsafeExecClassification(t *testing.T) {
	commands := []string{
		"kubectl -n stg exec deploy/gateway -- getent hosts grpc-user",
		"kubectl --context bc-stgdev exec deploy/gateway -- getent hosts grpc-user",
		"kubectl --context one --context two -n stg exec deploy/gateway -- getent hosts grpc-user",
		"kubectl --context bc-stgdev -n one --namespace two exec deploy/gateway -- getent hosts grpc-user",
		"kubectl --context= -n stg exec deploy/gateway -- getent hosts grpc-user",
		"kubectl --context bc-stgdev --namespace= exec deploy/gateway -- getent hosts grpc-user",
		"kubectl --kubeconfig /tmp/config --context bc-stgdev -n stg exec deploy/gateway -- getent hosts grpc-user",
		"kubectl --server https://cluster --context bc-stgdev -n stg exec deploy/gateway -- getent hosts grpc-user",
		"kubectl --token token --context bc-stgdev -n stg exec deploy/gateway -- getent hosts grpc-user",
		"kubectl --user admin --context bc-stgdev -n stg exec deploy/gateway -- getent hosts grpc-user",
		"kubectl --as root --context bc-stgdev -n stg exec deploy/gateway -- getent hosts grpc-user",
		"env kubectl --context bc-stgdev -n stg exec deploy/gateway -- getent hosts grpc-user",
		"sudo kubectl --context bc-stgdev -n stg exec deploy/gateway -- getent hosts grpc-user",
		"timeout 5 kubectl --context bc-stgdev -n stg exec deploy/gateway -- getent hosts grpc-user",
		"/usr/bin/kubectl --context bc-stgdev -n stg exec deploy/gateway -- getent hosts grpc-user",
		"kubectl --context bc-stgdev -n stg exec -it deploy/gateway -- getent hosts grpc-user",
		"kubectl --context bc-stgdev -n stg exec deploy/gateway -- getent hosts grpc-user && echo done",
		"kubectl --context bc-stgdev -n stg exec deploy/gateway -- getent hosts $(echo grpc-user)",
		"kubectl --context $CTX -n stg exec deploy/gateway -- getent hosts grpc-user",
		"kubectl --context bc-stgdev -n stg exec deploy/gateway -- getent hosts grpc-*",
		"kubectl --context bc-stgdev -n stg exec deploy/gateway -- cat /etc/resolv.conf < /tmp/input",
		"kubectl --context bc-stgdev -n stg exec deploy/gateway -- cat /etc/resolv.conf > /tmp/output",
		"kubectl --context bc-stgdev -n stg exec deploy/gateway -- env",
		"kubectl --context bc-stgdev -n stg exec deploy/gateway -- printenv",
		"kubectl --context bc-stgdev -n stg exec deploy/gateway -- cat /var/run/secrets/kubernetes.io/serviceaccount/token",
		"kubectl --context bc-stgdev -n stg exec deploy/gateway -- curl -s http://localhost:4191/metrics",
		"kubectl --context bc-stgdev -n stg exec deploy/gateway -- curl -fsS http://example.com",
		"kubectl --context bc-stgdev -n stg exec deploy/gateway -- dig grpc-user @8.8.8.8",
		"kubectl --context bc-stgdev -n stg exec deploy/gateway -- dig -f /tmp/names",
		"kubectl --context bc-stgdev -n stg exec deploy/gateway -- getent hosts grpc-user extra",
		"kubectl --context bc-stgdev -n stg exec deploy/gateway -- getent hosts -bad",
		"kubectl --context bc-stgdev -n stg exec deploy/gateway -- getent hosts bad_name",
		"kubectl --context bc-stgdev -n stg exec deploy/gateway -- /usr/bin/getent hosts grpc-user",
	}
	for _, command := range commands {
		t.Run(command, func(t *testing.T) {
			got := EvaluateGitOpsKubectl("Bash", command)
			if got.Decision != "ask" || got.LiveAccess != KubectlLiveAccessUnsafeExec || got.ExecScope != (KubectlExecScope{}) {
				t.Fatalf("unsafe classification = %+v", got)
			}
		})
	}
}

func TestEvaluateGitOpsKubectlPreservesOtherDecisions(t *testing.T) {
	portForward := EvaluateGitOpsKubectl("Bash", "kubectl -n stg port-forward svc/api 8080:80")
	if portForward.Decision != "ask" || portForward.LiveAccess != KubectlLiveAccessPortForward {
		t.Fatalf("port-forward = %+v", portForward)
	}
	mutation := EvaluateGitOpsKubectl("Bash", "kubectl delete pod api")
	if mutation.Decision != "block" || mutation.LiveAccess != KubectlLiveAccessNone || !strings.Contains(mutation.Reason, "GitOps") {
		t.Fatalf("mutation = %+v", mutation)
	}
	if got := EvaluateGitOpsKubectl("Bash", "kubectl get pods"); got.Decision != "" {
		t.Fatalf("read-only kubectl = %+v", got)
	}
	if got := EvaluateGitOpsKubectl("Read", "kubectl exec pod/api -- env"); got.Decision != "" {
		t.Fatalf("non-shell tool = %+v", got)
	}
}

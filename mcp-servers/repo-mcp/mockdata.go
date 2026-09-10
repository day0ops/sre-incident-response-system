package main

type RollbackResult struct {
	Deployment      string `json:"deployment"`
	RolledBackTo    string `json:"rolledBackTo"`
	Status          string `json:"status"`
	ReplicasHealthy string `json:"replicasHealthy"`
}

func mockRollback(deployment, version string) RollbackResult {
	return RollbackResult{
		Deployment:      deployment,
		RolledBackTo:    version,
		Status:          "rollback successful",
		ReplicasHealthy: "5/5 healthy",
	}
}

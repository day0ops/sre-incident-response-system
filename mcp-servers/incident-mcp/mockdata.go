package main

type Pod struct {
	Namespace string `json:"namespace"`
	Pod       string `json:"pod"`
	Status    string `json:"status"`
	Ready     bool   `json:"ready"`
	Reason    string `json:"reason"`
}

var mockPods = []Pod{
	{
		Namespace: "checkout",
		Pod:       "checkout-api-7d9f8c6b-a1b2c",
		Status:    "CrashLoopBackOff",
		Ready:     false,
		Reason:    "Readiness probe failed: cannot connect to postgres on checkout-db:5432",
	},
	{
		Namespace: "checkout",
		Pod:       "checkout-api-7d9f8c6b-d3e4f",
		Status:    "CrashLoopBackOff",
		Ready:     false,
		Reason:    "Readiness probe failed: cannot connect to postgres on checkout-db:5432",
	},
	{
		Namespace: "checkout",
		Pod:       "checkout-api-7d9f8c6b-g5h6i",
		Status:    "CrashLoopBackOff",
		Ready:     false,
		Reason:    "Readiness probe failed: cannot connect to postgres on checkout-db:5432",
	},
}

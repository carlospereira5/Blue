package loyverse_test

import (
	"context"
	"net/http"
	"testing"

	"blue/internal/loyverse"
)

func TestGetEmployees_Success(t *testing.T) {
	want := loyverse.EmployeesResponse{
		Employees: []loyverse.Employee{
			{ID: "e1", Name: "Juan", IsOwner: true},
			{ID: "e2", Name: "María", IsOwner: false},
		},
	}

	_, client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/employees" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(mustJSON(t, want))
	})

	got, err := client.GetEmployees(context.Background(), 50, "")
	if err != nil {
		t.Fatalf("GetEmployees() error = %v", err)
	}
	if len(got.Employees) != 2 {
		t.Errorf("got %d employees, want 2", len(got.Employees))
	}
	if got.Employees[0].Name != "Juan" {
		t.Errorf("Employees[0].Name = %q, want %q", got.Employees[0].Name, "Juan")
	}
}

func TestGetEmployeeByID_Success(t *testing.T) {
	want := loyverse.Employee{ID: "e1", Name: "Juan", IsOwner: true}

	_, client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/employees/e1" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(mustJSON(t, want))
	})

	got, err := client.GetEmployeeByID(context.Background(), "e1")
	if err != nil {
		t.Fatalf("GetEmployeeByID() error = %v", err)
	}
	if got.Name != "Juan" {
		t.Errorf("Name = %q, want %q", got.Name, "Juan")
	}
}

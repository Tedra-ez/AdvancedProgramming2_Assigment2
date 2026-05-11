package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"appointment-service/internal/model"
)

type fakeAppointmentRepo struct {
	createReturn model.Appointment
	createErr    error
	current      model.Appointment
	currentFound bool
	getErr       error
	updateReturn model.Appointment
	updateFound  bool
	updateErr    error
	createCalls  int
	getCalls     int
	updateCalls  int
}

func (r *fakeAppointmentRepo) Create(ctx context.Context, appointment model.Appointment) (model.Appointment, error) {
	r.createCalls++
	if r.createErr != nil {
		return model.Appointment{}, r.createErr
	}
	if r.createReturn.ID != "" {
		return r.createReturn, nil
	}
	appointment.ID = "appointment-1"
	appointment.CreatedAt = time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	appointment.UpdatedAt = appointment.CreatedAt
	return appointment, nil
}

func (r *fakeAppointmentRepo) GetByID(ctx context.Context, id string) (model.Appointment, bool, error) {
	r.getCalls++
	if r.getErr != nil {
		return model.Appointment{}, false, r.getErr
	}
	return r.current, r.currentFound, nil
}

func (r *fakeAppointmentRepo) List(context.Context) ([]model.Appointment, error) {
	return nil, nil
}

func (r *fakeAppointmentRepo) UpdateStatus(ctx context.Context, id string, status model.Status) (model.Appointment, bool, error) {
	r.updateCalls++
	if r.updateErr != nil {
		return model.Appointment{}, false, r.updateErr
	}
	if r.updateReturn.ID != "" {
		return r.updateReturn, r.updateFound, nil
	}
	updated := r.current
	updated.Status = status
	return updated, r.updateFound, nil
}

type fakeDoctorClient struct {
	exists bool
	err    error
	calls  int
	lastID string
}

func (c *fakeDoctorClient) Exists(ctx context.Context, doctorID string) (bool, error) {
	c.calls++
	c.lastID = doctorID
	return c.exists, c.err
}

type spyAppointmentPublisher struct {
	createCalls int
	statusCalls int
	created     model.Appointment
	statusID    string
	doctorID    string
	oldStatus   model.Status
	newStatus   model.Status
}

func (p *spyAppointmentPublisher) PublishAppointmentCreated(ctx context.Context, appointment model.Appointment) error {
	p.createCalls++
	p.created = appointment
	return nil
}

func (p *spyAppointmentPublisher) PublishAppointmentStatusUpdated(ctx context.Context, id string, doctorID string, oldStatus, newStatus model.Status) error {
	p.statusCalls++
	p.statusID = id
	p.doctorID = doctorID
	p.oldStatus = oldStatus
	p.newStatus = newStatus
	return nil
}

func (p *spyAppointmentPublisher) Close() error { return nil }

func TestCreatePublishesAppointmentCreatedAfterSuccessfulPersist(t *testing.T) {
	repo := &fakeAppointmentRepo{}
	doctors := &fakeDoctorClient{exists: true}
	pub := &spyAppointmentPublisher{}
	uc := NewAppointmentUseCase(repo, doctors, pub)

	created, err := uc.Create(context.Background(), model.Appointment{
		Title:       "Initial cardiac consultation",
		Description: "Patient referred for palpitations",
		DoctorID:    "doctor-1",
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if created.Status != model.StatusNew {
		t.Fatalf("created status = %q, want %q", created.Status, model.StatusNew)
	}
	if doctors.calls != 1 || doctors.lastID != "doctor-1" {
		t.Fatalf("doctor client calls = %d id = %q, want 1 doctor-1", doctors.calls, doctors.lastID)
	}
	if repo.createCalls != 1 {
		t.Fatalf("repo.Create calls = %d, want 1", repo.createCalls)
	}
	if pub.createCalls != 1 {
		t.Fatalf("PublishAppointmentCreated calls = %d, want 1", pub.createCalls)
	}
	if pub.created.ID != created.ID {
		t.Fatalf("published ID = %q, want %q", pub.created.ID, created.ID)
	}
}

func TestCreateDoesNotPersistOrPublishWhenDoctorMissing(t *testing.T) {
	repo := &fakeAppointmentRepo{}
	doctors := &fakeDoctorClient{exists: false}
	pub := &spyAppointmentPublisher{}
	uc := NewAppointmentUseCase(repo, doctors, pub)

	_, err := uc.Create(context.Background(), model.Appointment{
		Title:    "Follow-up visit",
		DoctorID: "doctor-missing",
	})
	if !errors.Is(err, ErrDoctorNotFound) {
		t.Fatalf("Create error = %v, want %v", err, ErrDoctorNotFound)
	}
	if repo.createCalls != 0 {
		t.Fatalf("repo.Create calls = %d, want 0", repo.createCalls)
	}
	if pub.createCalls != 0 {
		t.Fatalf("PublishAppointmentCreated calls = %d, want 0", pub.createCalls)
	}
}

func TestCreateReturnsUnavailableWhenDoctorClientFails(t *testing.T) {
	repo := &fakeAppointmentRepo{}
	doctors := &fakeDoctorClient{err: errors.New("connection refused")}
	pub := &spyAppointmentPublisher{}
	uc := NewAppointmentUseCase(repo, doctors, pub)

	_, err := uc.Create(context.Background(), model.Appointment{
		Title:    "Emergency consultation",
		DoctorID: "doctor-1",
	})
	if !errors.Is(err, ErrDoctorServiceUnavailable) {
		t.Fatalf("Create error = %v, want %v", err, ErrDoctorServiceUnavailable)
	}
	if repo.createCalls != 0 || pub.createCalls != 0 {
		t.Fatalf("repo.Create=%d publish=%d, want none", repo.createCalls, pub.createCalls)
	}
}

func TestUpdateStatusPublishesOldAndNewStatus(t *testing.T) {
	repo := &fakeAppointmentRepo{
		current: model.Appointment{
			ID:       "appointment-1",
			Title:    "Initial cardiac consultation",
			DoctorID: "doctor-1",
			Status:   model.StatusNew,
		},
		currentFound: true,
		updateReturn: model.Appointment{
			ID:       "appointment-1",
			Title:    "Initial cardiac consultation",
			DoctorID: "doctor-1",
			Status:   model.StatusInProgress,
		},
		updateFound: true,
	}
	doctors := &fakeDoctorClient{exists: true}
	pub := &spyAppointmentPublisher{}
	uc := NewAppointmentUseCase(repo, doctors, pub)

	updated, err := uc.UpdateStatus(context.Background(), "appointment-1", model.StatusInProgress)
	if err != nil {
		t.Fatalf("UpdateStatus returned error: %v", err)
	}
	if updated.Status != model.StatusInProgress {
		t.Fatalf("updated status = %q, want %q", updated.Status, model.StatusInProgress)
	}
	if repo.getCalls != 1 || repo.updateCalls != 1 {
		t.Fatalf("repo calls get=%d update=%d, want 1 each", repo.getCalls, repo.updateCalls)
	}
	if pub.statusCalls != 1 {
		t.Fatalf("PublishAppointmentStatusUpdated calls = %d, want 1", pub.statusCalls)
	}
	if pub.statusID != "appointment-1" || pub.doctorID != "doctor-1" || pub.oldStatus != model.StatusNew || pub.newStatus != model.StatusInProgress {
		t.Fatalf("published status event = id:%q doctor:%q old:%q new:%q", pub.statusID, pub.doctorID, pub.oldStatus, pub.newStatus)
	}
}

func TestUpdateStatusRejectsDoneToNewBeforePersistingOrPublishing(t *testing.T) {
	repo := &fakeAppointmentRepo{
		current: model.Appointment{
			ID:       "appointment-1",
			DoctorID: "doctor-1",
			Status:   model.StatusDone,
		},
		currentFound: true,
	}
	doctors := &fakeDoctorClient{exists: true}
	pub := &spyAppointmentPublisher{}
	uc := NewAppointmentUseCase(repo, doctors, pub)

	_, err := uc.UpdateStatus(context.Background(), "appointment-1", model.StatusNew)
	if !errors.Is(err, ErrInvalidStatusTransition) {
		t.Fatalf("UpdateStatus error = %v, want %v", err, ErrInvalidStatusTransition)
	}
	if doctors.calls != 0 || repo.updateCalls != 0 || pub.statusCalls != 0 {
		t.Fatalf("calls doctor=%d update=%d publish=%d, want none", doctors.calls, repo.updateCalls, pub.statusCalls)
	}
}

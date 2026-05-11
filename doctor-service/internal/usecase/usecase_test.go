package usecase

import (
	"context"
	"errors"
	"testing"

	"doctor-service/internal/model"
	"doctor-service/internal/repository"
)

type fakeDoctorRepo struct {
	existsByEmail bool
	existsErr     error
	createErr     error
	created       model.Doctor
	existsCalls   int
	createCalls   int
}

func (r *fakeDoctorRepo) Create(ctx context.Context, doctor model.Doctor) (model.Doctor, error) {
	r.createCalls++
	if r.createErr != nil {
		return model.Doctor{}, r.createErr
	}
	if r.created.ID != "" {
		return r.created, nil
	}
	doctor.ID = "doctor-1"
	return doctor, nil
}

func (r *fakeDoctorRepo) GetByID(context.Context, string) (model.Doctor, bool, error) {
	return model.Doctor{}, false, nil
}

func (r *fakeDoctorRepo) List(context.Context) ([]model.Doctor, error) {
	return nil, nil
}

func (r *fakeDoctorRepo) ExistsByEmail(context.Context, string) (bool, error) {
	r.existsCalls++
	return r.existsByEmail, r.existsErr
}

type spyDoctorPublisher struct {
	err   error
	calls int
	last  model.Doctor
}

func (p *spyDoctorPublisher) PublishDoctorCreated(ctx context.Context, doctor model.Doctor) error {
	p.calls++
	p.last = doctor
	return p.err
}

func (p *spyDoctorPublisher) Close() error { return nil }

func TestCreatePublishesDoctorCreatedAfterSuccessfulPersist(t *testing.T) {
	ctx := context.Background()
	repo := &fakeDoctorRepo{
		created: model.Doctor{
			ID:             "doctor-1",
			FullName:       "Dr. Aisha Seitkali",
			Specialization: "Cardiology",
			Email:          "a.seitkali@clinic.kz",
		},
	}
	pub := &spyDoctorPublisher{}
	uc := NewDoctorUseCase(repo, pub)

	created, err := uc.Create(ctx, model.Doctor{
		FullName:       "Dr. Aisha Seitkali",
		Specialization: "Cardiology",
		Email:          "a.seitkali@clinic.kz",
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if created.ID != "doctor-1" {
		t.Fatalf("created ID = %q, want doctor-1", created.ID)
	}
	if repo.createCalls != 1 {
		t.Fatalf("repo.Create calls = %d, want 1", repo.createCalls)
	}
	if pub.calls != 1 {
		t.Fatalf("PublishDoctorCreated calls = %d, want 1", pub.calls)
	}
	if pub.last.Email != created.Email {
		t.Fatalf("published email = %q, want %q", pub.last.Email, created.Email)
	}
}

func TestCreateDoesNotPublishWhenValidationFails(t *testing.T) {
	repo := &fakeDoctorRepo{}
	pub := &spyDoctorPublisher{}
	uc := NewDoctorUseCase(repo, pub)

	_, err := uc.Create(context.Background(), model.Doctor{Email: "a.seitkali@clinic.kz"})
	if !errors.Is(err, ErrFullNameRequired) {
		t.Fatalf("Create error = %v, want %v", err, ErrFullNameRequired)
	}
	if repo.existsCalls != 0 || repo.createCalls != 0 {
		t.Fatalf("repo calls = exists:%d create:%d, want none", repo.existsCalls, repo.createCalls)
	}
	if pub.calls != 0 {
		t.Fatalf("PublishDoctorCreated calls = %d, want 0", pub.calls)
	}
}

func TestCreateMapsDuplicateEmailAndDoesNotPublish(t *testing.T) {
	repo := &fakeDoctorRepo{createErr: repository.ErrDuplicateEmail}
	pub := &spyDoctorPublisher{}
	uc := NewDoctorUseCase(repo, pub)

	_, err := uc.Create(context.Background(), model.Doctor{
		FullName: "Dr. Aisha Seitkali",
		Email:    "a.seitkali@clinic.kz",
	})
	if !errors.Is(err, ErrEmailNotUnique) {
		t.Fatalf("Create error = %v, want %v", err, ErrEmailNotUnique)
	}
	if pub.calls != 0 {
		t.Fatalf("PublishDoctorCreated calls = %d, want 0", pub.calls)
	}
}

func TestCreateIgnoresPublisherError(t *testing.T) {
	repo := &fakeDoctorRepo{created: model.Doctor{
		ID:       "doctor-1",
		FullName: "Dr. Aisha Seitkali",
		Email:    "a.seitkali@clinic.kz",
	}}
	pub := &spyDoctorPublisher{err: errors.New("nats down")}
	uc := NewDoctorUseCase(repo, pub)

	created, err := uc.Create(context.Background(), model.Doctor{
		FullName: "Dr. Aisha Seitkali",
		Email:    "a.seitkali@clinic.kz",
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if created.ID != "doctor-1" {
		t.Fatalf("created ID = %q, want doctor-1", created.ID)
	}
	if pub.calls != 1 {
		t.Fatalf("PublishDoctorCreated calls = %d, want 1", pub.calls)
	}
}

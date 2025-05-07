package database_test

import (
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/lunagic/athena/athenaservices/database"
	"gotest.tools/v3/assert"
)

type UserSettings struct {
	FavoriteColor string
}

type UserID int64
type CompanyID int64

type ResourceFromOtherSystem struct {
	ID      int64  `db:"id,primaryKey"`
	Subject string `db:"subject"`
}

func (e ResourceFromOtherSystem) TableStructure() database.Table {
	return database.Table{
		Name: "resource_from_other_system",
	}
}

type Company struct {
	ID CompanyID `db:"id,primaryKey,autoIncrement"`
	//
	String   string    `db:"name"`
	Bool     bool      `db:"bool"`
	Int      int       `db:"int"`
	Int8     int8      `db:"int8"`
	Int16    int16     `db:"int16"`
	Int32    int32     `db:"int32"`
	Int64    int64     `db:"int64"`
	Uint     uint      `db:"uint"`
	Uint8    uint8     `db:"uint8"`
	Uint16   uint16    `db:"uint16"`
	Uint32   uint32    `db:"uint32"`
	Uint64   uint64    `db:"uint64"`
	Float32  float32   `db:"float32"`
	Float64  float64   `db:"float64"`
	TimeTime time.Time `db:"timeTime"`
	Struct   struct{}  `db:"struct"`
	Slice    []string  `db:"slice"`
}

func (e Company) TableStructure() database.Table {
	return database.Table{
		Name: "company",
	}
}

type UserV1 struct {
	ID                UserID       `db:"id,primaryKey,autoIncrement"`
	Email             string       `db:"email_address,comment=this is the comment"`
	CompanyID         CompanyID    `db:"company_id,foreignKey=company.id"`
	CreatedAt         time.Time    `db:"created_at,readOnly,default=CURRENT_TIMESTAMP"`
	Settings          UserSettings `db:"settings"`
	WillBeRemovedInV2 *string      `db:"will_be_removed_in_v2"`
	WillBeChangedInV2 string       `db:"will_be_changed_in_v2"`
}

func (e UserV1) TableStructure() database.Table {
	return database.Table{
		Name:    "user",
		Comment: "foobar",
		Indexes: []database.TableIndex{
			{
				Name:    "ix_user_company_id",
				Columns: []string{"company_id"},
				Unique:  false,
			},
			{
				Name:    "ix_user_email_and_company_that_will_change_in_v2",
				Columns: []string{"email_address", "company_id"},
				Unique:  false,
			},
			{
				Name:    "ix_user_email_and_company_that_will_be_removed_in_v2",
				Columns: []string{"email_address"},
				Unique:  false,
			},
		},
	}
}

type UserV2 struct {
	ID                UserID       `db:"id,primaryKey,autoIncrement"`
	Email             string       `db:"email_address"`
	CreatedAt         time.Time    `db:"created_at,readOnly,default=CURRENT_TIMESTAMP"`
	Settings          UserSettings `db:"settings"`
	CompanyID         CompanyID    `db:"company_id,foreignKey=company.id"`
	NewForV2          string       `db:"NewForV2"`
	WillBeChangedInV2 *string      `db:"WillBeChangedInV2"`
}

func (e UserV2) TableStructure() database.Table {
	return database.Table{
		Name:    "user",
		Comment: "foobar",
		Indexes: []database.TableIndex{
			{
				Name:    "ix_user_id",
				Columns: []string{"id"},
				Unique:  true,
			},
			{
				Name:    "ix_user_company_id",
				Columns: []string{"company_id"},
				Unique:  false,
			},
			{
				Name:    "ix_user_email_and_company_that_will_change_in_v2",
				Columns: []string{"email_address", "company_id"},
				Unique:  false,
			},
		},
	}
}

func testSuite(t *testing.T, driver database.Driver, configFuncs ...database.ServiceConfigFunc) {
	configFuncs = append(configFuncs, database.WithLogger(slog.Default()))
	service, err := database.New(driver, configFuncs...)
	assert.NilError(t, err)

	migrationInputRound1 := []database.Entity{
		ResourceFromOtherSystem{},
		Company{},
		UserV1{},
	}

	{ // Assert that the migration actually made changes
		numberOfChanges, err := service.AutoMigrate(t.Context(), migrationInputRound1)
		assert.NilError(t, err)
		assert.Assert(t, numberOfChanges != 0)
	}

	{ // Assert that running the same migration again does not result in any changes
		numberOfChanges, err := service.AutoMigrate(t.Context(), migrationInputRound1)
		assert.NilError(t, err)
		assert.Assert(t, numberOfChanges == 0)
	}

	migrationInputRound2 := []database.Entity{
		ResourceFromOtherSystem{},
		Company{},
		UserV2{},
	}

	{ // Assert more migration changes
		{ // Assert that the migration actually made changes
			numberOfChanges, err := service.AutoMigrate(t.Context(), migrationInputRound2)
			assert.NilError(t, err)
			assert.Assert(t, numberOfChanges != 0)
		}
	}

	{ // Assert that running the same migration again does not result in any changes
		numberOfChanges, err := service.AutoMigrate(t.Context(), migrationInputRound2)
		assert.NilError(t, err)
		assert.Assert(t, numberOfChanges == 0)
	}

	userRepo := database.NewRepository[UserID, UserV2](service)
	companyRepo := database.NewRepository[CompanyID, Company](service)
	resourceFromOtherSystemRepo := database.NewRepository[int64, ResourceFromOtherSystem](service)
	{ // Assert that a user crud methods work
		testEmailAddress := uuid.NewString()
		testFavoriteColor := uuid.NewString()

		companyID, err := companyRepo.Insert(t.Context(), Company{
			TimeTime: time.Now(),
		})
		if err != nil {
			assert.NilError(t, err)
		}

		{ // Create a new user
			newUserID, err := userRepo.Insert(t.Context(), UserV2{
				Email:     testEmailAddress,
				CompanyID: companyID,
				Settings:  UserSettings{FavoriteColor: testFavoriteColor},
			})
			assert.NilError(t, err)
			assert.Equal(t, newUserID, UserID(1))
		}

		{ // Get the user by ID
			newUser, err := userRepo.SelectSingle(t.Context(), database.WithAdditionalWhere(
				database.And(
					database.Equal(&userRepo.T.ID, 1),
				),
			))
			assert.NilError(t, err)
			assert.Equal(t, newUser.ID, UserID(1))
			assert.Equal(t, newUser.Email, testEmailAddress)
			assert.Equal(t, newUser.Settings.FavoriteColor, testFavoriteColor)
		}

		{ // Update
			newEmailAddress := uuid.NewString()
			err := userRepo.Update(t.Context(), UserV2{
				ID:        1,
				Email:     newEmailAddress,
				CompanyID: companyID,
			})
			assert.NilError(t, err)

			user, err := userRepo.SelectSingle(t.Context(), database.WithAdditionalWhere(
				database.And(
					database.Equal(&userRepo.T.ID, 1),
				),
			))
			assert.NilError(t, err)
			assert.Equal(t, user.Email, newEmailAddress)
		}

		{ // Delete
			err := userRepo.Delete(t.Context(), UserV2{ID: 1})
			assert.NilError(t, err)
		}

		{ // Read again to confirm it's gone
			_, err := userRepo.SelectSingle(t.Context(), database.WithAdditionalWhere(
				database.And(
					database.Equal(&userRepo.T.ID, 1),
				),
			))
			assert.ErrorIs(t, err, database.ErrNoRows)
		}

		{ // Test non-auto incrementing primary key
			expectedID := int64(42)
			actualID, err := resourceFromOtherSystemRepo.Insert(
				t.Context(),
				ResourceFromOtherSystem{
					ID:      expectedID,
					Subject: uuid.NewString(),
				},
			)
			assert.NilError(t, err)
			assert.Equal(t, actualID, expectedID)
		}
	}

	{ // Assert that foreign key relationships work when deleting
		companyID, err := companyRepo.Insert(t.Context(), Company{
			TimeTime: time.Now(),
		})
		assert.NilError(t, err)

		newUserID, err := userRepo.Insert(t.Context(), UserV2{
			Email:     uuid.NewString(),
			CompanyID: companyID,
		})
		assert.NilError(t, err)

		user, err := userRepo.SelectSingle(t.Context(), database.WithAdditionalWhere(
			database.And(
				database.Equal(&userRepo.T.ID, newUserID),
			),
		))
		assert.NilError(t, err)

		if err := companyRepo.Delete(t.Context(), Company{ID: companyID}); err != nil {
			assert.NilError(t, err)
		}

		_, errShouldBeNoRows := userRepo.SelectSingle(t.Context(), database.WithAdditionalWhere(
			database.And(
				database.Equal(&userRepo.T.ID, user.ID),
			),
		))
		assert.ErrorIs(t, errShouldBeNoRows, database.ErrNoRows)
	}
}

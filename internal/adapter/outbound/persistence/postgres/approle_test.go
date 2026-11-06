package postgres_test

import (
	"testing"

	"gorm.io/gorm"
)

func TestAppRoleReadsAndWritesDataButNotSchema(t *testing.T) {
	err := testDB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SET LOCAL ROLE authenticity_app").Error; err != nil {
			return err
		}
		for _, sql := range []string{
			"SELECT count(*) FROM usr.vw_user_effective_permissions",
			"SELECT count(*) FROM sys.outbox",
			"DELETE FROM usr.usr_session WHERE false",
			"UPDATE app.product SET name = name WHERE false",
		} {
			if err := tx.Exec(sql).Error; err != nil {
				t.Errorf("%s: %v", sql, err)
				return nil
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	err = testDB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SET LOCAL ROLE authenticity_app").Error; err != nil {
			t.Fatal(err)
		}
		return tx.Exec("DROP TABLE app.product").Error
	})
	if err == nil {
		t.Fatal("app role dropped a table")
	}
}

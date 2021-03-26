package connection

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/lib/pq"
	"k8s.io/klog"
)

// Create creates connection with postgres db
func Create(filename string) (*sql.DB, *pq.Listener, error) {
	config, err := getDbConfig(filename)
	if err != nil {
		return nil, nil, err
	}

	pgConn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		config.Addr,
		config.Port,
		config.Username,
		config.Password,
		config.DBName,
	)

	listener := pq.NewListener(pgConn, 5*time.Second, 20*time.Second, nil)

	db, err := sql.Open("postgres", pgConn)
	if err != nil {
		return nil, nil, err
	}

	if err = db.Ping(); err != nil {
		return nil, nil, err
	}

	klog.Info("Successfully connected!")
	return db, listener, nil
}

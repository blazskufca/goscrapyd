// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.27.0

package database

import (
	"context"
	"database/sql"
	"fmt"
)

type DBTX interface {
	ExecContext(context.Context, string, ...interface{}) (sql.Result, error)
	PrepareContext(context.Context, string) (*sql.Stmt, error)
	QueryContext(context.Context, string, ...interface{}) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...interface{}) *sql.Row
}

func New(db DBTX) *Queries {
	return &Queries{db: db}
}

func Prepare(ctx context.Context, db DBTX) (*Queries, error) {
	q := Queries{db: db}
	var err error
	if q.checkSettingsExistStmt, err = db.PrepareContext(ctx, checkSettingsExist); err != nil {
		return nil, fmt.Errorf("error preparing query CheckSettingsExist: %w", err)
	}
	if q.createNewUserStmt, err = db.PrepareContext(ctx, createNewUser); err != nil {
		return nil, fmt.Errorf("error preparing query CreateNewUser: %w", err)
	}
	if q.deleteScrapydNodesStmt, err = db.PrepareContext(ctx, deleteScrapydNodes); err != nil {
		return nil, fmt.Errorf("error preparing query DeleteScrapydNodes: %w", err)
	}
	if q.deleteTaskWhereUUIDStmt, err = db.PrepareContext(ctx, deleteTaskWhereUUID); err != nil {
		return nil, fmt.Errorf("error preparing query DeleteTaskWhereUUID: %w", err)
	}
	if q.deleteUserByUUIDStmt, err = db.PrepareContext(ctx, deleteUserByUUID); err != nil {
		return nil, fmt.Errorf("error preparing query DeleteUserByUUID: %w", err)
	}
	if q.getAllUsersStmt, err = db.PrepareContext(ctx, getAllUsers); err != nil {
		return nil, fmt.Errorf("error preparing query GetAllUsers: %w", err)
	}
	if q.getJobsForNodeStmt, err = db.PrepareContext(ctx, getJobsForNode); err != nil {
		return nil, fmt.Errorf("error preparing query GetJobsForNode: %w", err)
	}
	if q.getNodeWithNameStmt, err = db.PrepareContext(ctx, getNodeWithName); err != nil {
		return nil, fmt.Errorf("error preparing query GetNodeWithName: %w", err)
	}
	if q.getSettingsStmt, err = db.PrepareContext(ctx, getSettings); err != nil {
		return nil, fmt.Errorf("error preparing query GetSettings: %w", err)
	}
	if q.getTaskWithUUIDStmt, err = db.PrepareContext(ctx, getTaskWithUUID); err != nil {
		return nil, fmt.Errorf("error preparing query GetTaskWithUUID: %w", err)
	}
	if q.getTasksStmt, err = db.PrepareContext(ctx, getTasks); err != nil {
		return nil, fmt.Errorf("error preparing query GetTasks: %w", err)
	}
	if q.getTasksWithLatestJobMetadataStmt, err = db.PrepareContext(ctx, getTasksWithLatestJobMetadata); err != nil {
		return nil, fmt.Errorf("error preparing query GetTasksWithLatestJobMetadata: %w", err)
	}
	if q.getTotalJobCountForNodeStmt, err = db.PrepareContext(ctx, getTotalJobCountForNode); err != nil {
		return nil, fmt.Errorf("error preparing query GetTotalJobCountForNode: %w", err)
	}
	if q.getUserByUsernameStmt, err = db.PrepareContext(ctx, getUserByUsername); err != nil {
		return nil, fmt.Errorf("error preparing query GetUserByUsername: %w", err)
	}
	if q.getUserWithIDStmt, err = db.PrepareContext(ctx, getUserWithID); err != nil {
		return nil, fmt.Errorf("error preparing query GetUserWithID: %w", err)
	}
	if q.insertJobStmt, err = db.PrepareContext(ctx, insertJob); err != nil {
		return nil, fmt.Errorf("error preparing query InsertJob: %w", err)
	}
	if q.insertSettingsStmt, err = db.PrepareContext(ctx, insertSettings); err != nil {
		return nil, fmt.Errorf("error preparing query InsertSettings: %w", err)
	}
	if q.insertTaskStmt, err = db.PrepareContext(ctx, insertTask); err != nil {
		return nil, fmt.Errorf("error preparing query InsertTask: %w", err)
	}
	if q.listScrapydNodesStmt, err = db.PrepareContext(ctx, listScrapydNodes); err != nil {
		return nil, fmt.Errorf("error preparing query ListScrapydNodes: %w", err)
	}
	if q.newScrapydNodeStmt, err = db.PrepareContext(ctx, newScrapydNode); err != nil {
		return nil, fmt.Errorf("error preparing query NewScrapydNode: %w", err)
	}
	if q.searchNodeJobsStmt, err = db.PrepareContext(ctx, searchNodeJobs); err != nil {
		return nil, fmt.Errorf("error preparing query SearchNodeJobs: %w", err)
	}
	if q.searchTasksTableStmt, err = db.PrepareContext(ctx, searchTasksTable); err != nil {
		return nil, fmt.Errorf("error preparing query SearchTasksTable: %w", err)
	}
	if q.setErrorWhereJobIdStmt, err = db.PrepareContext(ctx, setErrorWhereJobId); err != nil {
		return nil, fmt.Errorf("error preparing query SetErrorWhereJobId: %w", err)
	}
	if q.setStoppedByOnJobStmt, err = db.PrepareContext(ctx, setStoppedByOnJob); err != nil {
		return nil, fmt.Errorf("error preparing query SetStoppedByOnJob: %w", err)
	}
	if q.softDeleteJobStmt, err = db.PrepareContext(ctx, softDeleteJob); err != nil {
		return nil, fmt.Errorf("error preparing query SoftDeleteJob: %w", err)
	}
	if q.startFinishRuntimeLogsItemsForJobWithJobIDStmt, err = db.PrepareContext(ctx, startFinishRuntimeLogsItemsForJobWithJobID); err != nil {
		return nil, fmt.Errorf("error preparing query StartFinishRuntimeLogsItemsForJobWithJobID: %w", err)
	}
	if q.updateNodeWhereNameStmt, err = db.PrepareContext(ctx, updateNodeWhereName); err != nil {
		return nil, fmt.Errorf("error preparing query UpdateNodeWhereName: %w", err)
	}
	if q.updateSettingsStmt, err = db.PrepareContext(ctx, updateSettings); err != nil {
		return nil, fmt.Errorf("error preparing query UpdateSettings: %w", err)
	}
	if q.updateTaskStmt, err = db.PrepareContext(ctx, updateTask); err != nil {
		return nil, fmt.Errorf("error preparing query UpdateTask: %w", err)
	}
	if q.updateTaskPausedStmt, err = db.PrepareContext(ctx, updateTaskPaused); err != nil {
		return nil, fmt.Errorf("error preparing query UpdateTaskPaused: %w", err)
	}
	if q.updateUserWhereUUIDStmt, err = db.PrepareContext(ctx, updateUserWhereUUID); err != nil {
		return nil, fmt.Errorf("error preparing query UpdateUserWhereUUID: %w", err)
	}
	if q.updateUsersPasswordWhereIDStmt, err = db.PrepareContext(ctx, updateUsersPasswordWhereID); err != nil {
		return nil, fmt.Errorf("error preparing query UpdateUsersPasswordWhereID: %w", err)
	}
	return &q, nil
}

func (q *Queries) Close() error {
	var err error
	if q.checkSettingsExistStmt != nil {
		if cerr := q.checkSettingsExistStmt.Close(); cerr != nil {
			err = fmt.Errorf("error closing checkSettingsExistStmt: %w", cerr)
		}
	}
	if q.createNewUserStmt != nil {
		if cerr := q.createNewUserStmt.Close(); cerr != nil {
			err = fmt.Errorf("error closing createNewUserStmt: %w", cerr)
		}
	}
	if q.deleteScrapydNodesStmt != nil {
		if cerr := q.deleteScrapydNodesStmt.Close(); cerr != nil {
			err = fmt.Errorf("error closing deleteScrapydNodesStmt: %w", cerr)
		}
	}
	if q.deleteTaskWhereUUIDStmt != nil {
		if cerr := q.deleteTaskWhereUUIDStmt.Close(); cerr != nil {
			err = fmt.Errorf("error closing deleteTaskWhereUUIDStmt: %w", cerr)
		}
	}
	if q.deleteUserByUUIDStmt != nil {
		if cerr := q.deleteUserByUUIDStmt.Close(); cerr != nil {
			err = fmt.Errorf("error closing deleteUserByUUIDStmt: %w", cerr)
		}
	}
	if q.getAllUsersStmt != nil {
		if cerr := q.getAllUsersStmt.Close(); cerr != nil {
			err = fmt.Errorf("error closing getAllUsersStmt: %w", cerr)
		}
	}
	if q.getJobsForNodeStmt != nil {
		if cerr := q.getJobsForNodeStmt.Close(); cerr != nil {
			err = fmt.Errorf("error closing getJobsForNodeStmt: %w", cerr)
		}
	}
	if q.getNodeWithNameStmt != nil {
		if cerr := q.getNodeWithNameStmt.Close(); cerr != nil {
			err = fmt.Errorf("error closing getNodeWithNameStmt: %w", cerr)
		}
	}
	if q.getSettingsStmt != nil {
		if cerr := q.getSettingsStmt.Close(); cerr != nil {
			err = fmt.Errorf("error closing getSettingsStmt: %w", cerr)
		}
	}
	if q.getTaskWithUUIDStmt != nil {
		if cerr := q.getTaskWithUUIDStmt.Close(); cerr != nil {
			err = fmt.Errorf("error closing getTaskWithUUIDStmt: %w", cerr)
		}
	}
	if q.getTasksStmt != nil {
		if cerr := q.getTasksStmt.Close(); cerr != nil {
			err = fmt.Errorf("error closing getTasksStmt: %w", cerr)
		}
	}
	if q.getTasksWithLatestJobMetadataStmt != nil {
		if cerr := q.getTasksWithLatestJobMetadataStmt.Close(); cerr != nil {
			err = fmt.Errorf("error closing getTasksWithLatestJobMetadataStmt: %w", cerr)
		}
	}
	if q.getTotalJobCountForNodeStmt != nil {
		if cerr := q.getTotalJobCountForNodeStmt.Close(); cerr != nil {
			err = fmt.Errorf("error closing getTotalJobCountForNodeStmt: %w", cerr)
		}
	}
	if q.getUserByUsernameStmt != nil {
		if cerr := q.getUserByUsernameStmt.Close(); cerr != nil {
			err = fmt.Errorf("error closing getUserByUsernameStmt: %w", cerr)
		}
	}
	if q.getUserWithIDStmt != nil {
		if cerr := q.getUserWithIDStmt.Close(); cerr != nil {
			err = fmt.Errorf("error closing getUserWithIDStmt: %w", cerr)
		}
	}
	if q.insertJobStmt != nil {
		if cerr := q.insertJobStmt.Close(); cerr != nil {
			err = fmt.Errorf("error closing insertJobStmt: %w", cerr)
		}
	}
	if q.insertSettingsStmt != nil {
		if cerr := q.insertSettingsStmt.Close(); cerr != nil {
			err = fmt.Errorf("error closing insertSettingsStmt: %w", cerr)
		}
	}
	if q.insertTaskStmt != nil {
		if cerr := q.insertTaskStmt.Close(); cerr != nil {
			err = fmt.Errorf("error closing insertTaskStmt: %w", cerr)
		}
	}
	if q.listScrapydNodesStmt != nil {
		if cerr := q.listScrapydNodesStmt.Close(); cerr != nil {
			err = fmt.Errorf("error closing listScrapydNodesStmt: %w", cerr)
		}
	}
	if q.newScrapydNodeStmt != nil {
		if cerr := q.newScrapydNodeStmt.Close(); cerr != nil {
			err = fmt.Errorf("error closing newScrapydNodeStmt: %w", cerr)
		}
	}
	if q.searchNodeJobsStmt != nil {
		if cerr := q.searchNodeJobsStmt.Close(); cerr != nil {
			err = fmt.Errorf("error closing searchNodeJobsStmt: %w", cerr)
		}
	}
	if q.searchTasksTableStmt != nil {
		if cerr := q.searchTasksTableStmt.Close(); cerr != nil {
			err = fmt.Errorf("error closing searchTasksTableStmt: %w", cerr)
		}
	}
	if q.setErrorWhereJobIdStmt != nil {
		if cerr := q.setErrorWhereJobIdStmt.Close(); cerr != nil {
			err = fmt.Errorf("error closing setErrorWhereJobIdStmt: %w", cerr)
		}
	}
	if q.setStoppedByOnJobStmt != nil {
		if cerr := q.setStoppedByOnJobStmt.Close(); cerr != nil {
			err = fmt.Errorf("error closing setStoppedByOnJobStmt: %w", cerr)
		}
	}
	if q.softDeleteJobStmt != nil {
		if cerr := q.softDeleteJobStmt.Close(); cerr != nil {
			err = fmt.Errorf("error closing softDeleteJobStmt: %w", cerr)
		}
	}
	if q.startFinishRuntimeLogsItemsForJobWithJobIDStmt != nil {
		if cerr := q.startFinishRuntimeLogsItemsForJobWithJobIDStmt.Close(); cerr != nil {
			err = fmt.Errorf("error closing startFinishRuntimeLogsItemsForJobWithJobIDStmt: %w", cerr)
		}
	}
	if q.updateNodeWhereNameStmt != nil {
		if cerr := q.updateNodeWhereNameStmt.Close(); cerr != nil {
			err = fmt.Errorf("error closing updateNodeWhereNameStmt: %w", cerr)
		}
	}
	if q.updateSettingsStmt != nil {
		if cerr := q.updateSettingsStmt.Close(); cerr != nil {
			err = fmt.Errorf("error closing updateSettingsStmt: %w", cerr)
		}
	}
	if q.updateTaskStmt != nil {
		if cerr := q.updateTaskStmt.Close(); cerr != nil {
			err = fmt.Errorf("error closing updateTaskStmt: %w", cerr)
		}
	}
	if q.updateTaskPausedStmt != nil {
		if cerr := q.updateTaskPausedStmt.Close(); cerr != nil {
			err = fmt.Errorf("error closing updateTaskPausedStmt: %w", cerr)
		}
	}
	if q.updateUserWhereUUIDStmt != nil {
		if cerr := q.updateUserWhereUUIDStmt.Close(); cerr != nil {
			err = fmt.Errorf("error closing updateUserWhereUUIDStmt: %w", cerr)
		}
	}
	if q.updateUsersPasswordWhereIDStmt != nil {
		if cerr := q.updateUsersPasswordWhereIDStmt.Close(); cerr != nil {
			err = fmt.Errorf("error closing updateUsersPasswordWhereIDStmt: %w", cerr)
		}
	}
	return err
}

func (q *Queries) exec(ctx context.Context, stmt *sql.Stmt, query string, args ...interface{}) (sql.Result, error) {
	switch {
	case stmt != nil && q.tx != nil:
		return q.tx.StmtContext(ctx, stmt).ExecContext(ctx, args...)
	case stmt != nil:
		return stmt.ExecContext(ctx, args...)
	default:
		return q.db.ExecContext(ctx, query, args...)
	}
}

func (q *Queries) query(ctx context.Context, stmt *sql.Stmt, query string, args ...interface{}) (*sql.Rows, error) {
	switch {
	case stmt != nil && q.tx != nil:
		return q.tx.StmtContext(ctx, stmt).QueryContext(ctx, args...)
	case stmt != nil:
		return stmt.QueryContext(ctx, args...)
	default:
		return q.db.QueryContext(ctx, query, args...)
	}
}

func (q *Queries) queryRow(ctx context.Context, stmt *sql.Stmt, query string, args ...interface{}) *sql.Row {
	switch {
	case stmt != nil && q.tx != nil:
		return q.tx.StmtContext(ctx, stmt).QueryRowContext(ctx, args...)
	case stmt != nil:
		return stmt.QueryRowContext(ctx, args...)
	default:
		return q.db.QueryRowContext(ctx, query, args...)
	}
}

type Queries struct {
	db                                             DBTX
	tx                                             *sql.Tx
	checkSettingsExistStmt                         *sql.Stmt
	createNewUserStmt                              *sql.Stmt
	deleteScrapydNodesStmt                         *sql.Stmt
	deleteTaskWhereUUIDStmt                        *sql.Stmt
	deleteUserByUUIDStmt                           *sql.Stmt
	getAllUsersStmt                                *sql.Stmt
	getJobsForNodeStmt                             *sql.Stmt
	getNodeWithNameStmt                            *sql.Stmt
	getSettingsStmt                                *sql.Stmt
	getTaskWithUUIDStmt                            *sql.Stmt
	getTasksStmt                                   *sql.Stmt
	getTasksWithLatestJobMetadataStmt              *sql.Stmt
	getTotalJobCountForNodeStmt                    *sql.Stmt
	getUserByUsernameStmt                          *sql.Stmt
	getUserWithIDStmt                              *sql.Stmt
	insertJobStmt                                  *sql.Stmt
	insertSettingsStmt                             *sql.Stmt
	insertTaskStmt                                 *sql.Stmt
	listScrapydNodesStmt                           *sql.Stmt
	newScrapydNodeStmt                             *sql.Stmt
	searchNodeJobsStmt                             *sql.Stmt
	searchTasksTableStmt                           *sql.Stmt
	setErrorWhereJobIdStmt                         *sql.Stmt
	setStoppedByOnJobStmt                          *sql.Stmt
	softDeleteJobStmt                              *sql.Stmt
	startFinishRuntimeLogsItemsForJobWithJobIDStmt *sql.Stmt
	updateNodeWhereNameStmt                        *sql.Stmt
	updateSettingsStmt                             *sql.Stmt
	updateTaskStmt                                 *sql.Stmt
	updateTaskPausedStmt                           *sql.Stmt
	updateUserWhereUUIDStmt                        *sql.Stmt
	updateUsersPasswordWhereIDStmt                 *sql.Stmt
}

func (q *Queries) WithTx(tx *sql.Tx) *Queries {
	return &Queries{
		db:                                tx,
		tx:                                tx,
		checkSettingsExistStmt:            q.checkSettingsExistStmt,
		createNewUserStmt:                 q.createNewUserStmt,
		deleteScrapydNodesStmt:            q.deleteScrapydNodesStmt,
		deleteTaskWhereUUIDStmt:           q.deleteTaskWhereUUIDStmt,
		deleteUserByUUIDStmt:              q.deleteUserByUUIDStmt,
		getAllUsersStmt:                   q.getAllUsersStmt,
		getJobsForNodeStmt:                q.getJobsForNodeStmt,
		getNodeWithNameStmt:               q.getNodeWithNameStmt,
		getSettingsStmt:                   q.getSettingsStmt,
		getTaskWithUUIDStmt:               q.getTaskWithUUIDStmt,
		getTasksStmt:                      q.getTasksStmt,
		getTasksWithLatestJobMetadataStmt: q.getTasksWithLatestJobMetadataStmt,
		getTotalJobCountForNodeStmt:       q.getTotalJobCountForNodeStmt,
		getUserByUsernameStmt:             q.getUserByUsernameStmt,
		getUserWithIDStmt:                 q.getUserWithIDStmt,
		insertJobStmt:                     q.insertJobStmt,
		insertSettingsStmt:                q.insertSettingsStmt,
		insertTaskStmt:                    q.insertTaskStmt,
		listScrapydNodesStmt:              q.listScrapydNodesStmt,
		newScrapydNodeStmt:                q.newScrapydNodeStmt,
		searchNodeJobsStmt:                q.searchNodeJobsStmt,
		searchTasksTableStmt:              q.searchTasksTableStmt,
		setErrorWhereJobIdStmt:            q.setErrorWhereJobIdStmt,
		setStoppedByOnJobStmt:             q.setStoppedByOnJobStmt,
		softDeleteJobStmt:                 q.softDeleteJobStmt,
		startFinishRuntimeLogsItemsForJobWithJobIDStmt: q.startFinishRuntimeLogsItemsForJobWithJobIDStmt,
		updateNodeWhereNameStmt:                        q.updateNodeWhereNameStmt,
		updateSettingsStmt:                             q.updateSettingsStmt,
		updateTaskStmt:                                 q.updateTaskStmt,
		updateTaskPausedStmt:                           q.updateTaskPausedStmt,
		updateUserWhereUUIDStmt:                        q.updateUserWhereUUIDStmt,
		updateUsersPasswordWhereIDStmt:                 q.updateUsersPasswordWhereIDStmt,
	}
}

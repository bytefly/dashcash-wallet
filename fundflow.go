package main

import (
	"database/sql"
	"errors"
	"fmt"
	conf "github.com/bytefly/dashcash-wallet/config"
	"github.com/bytefly/dashcash-wallet/util"
	_ "github.com/go-sql-driver/mysql"
	"log"
	"math/rand"
	"strconv"
	"strings"
	"time"
)

var coinIds = make(map[string]int)

func init() {
	rand.Seed(time.Now().UnixNano())
}

func EnterFundflowDB(config *conf.Config, message NotifyMessage, symbol, fee string) (err error) {
	switch message.TxType {
	case TYPE_FUND_COLLECTION: //fund collection
		err = saveInnerTxFlow(config, message, symbol, fee)
	case TYPE_ADMIN_DEPOSIT: //admin deposit
		err = saveInnerTxFlow(config, message, symbol, fee)
	case TYPE_USER_DEPOSIT: // user deposit
		err = saveDepositTxFlow(config, message, symbol, fee)
	case TYPE_USER_WITHDRAW: //user withdraw
		err = saveUserWithdrawTxFlow(config, message, symbol, fee)
	case TYPE_ADMIN_WITHDRAW: //admin withdraw
		err = saveAdminWithdrawTxFlow(config, message, symbol, fee)
	default:
		log.Println("transaction not belong to us")
	}

	if err != nil {
		log.Println("save fund flow fail, tx:", message.TxHash)
	}
	return
}

func generateFlowID(message NotifyMessage, userID int) string {
	t := time.Unix(int64(message.BlockTime), 0)
	return fmt.Sprintf("%04d%02d%02d%07d%02d%02d%02d%04d", t.Year(), t.Month(), t.Day(), userID, t.Hour(), t.Minute(), t.Second(), rand.Intn(10000))
}

func saveInnerTxFlow(config *conf.Config, message NotifyMessage, symbol, fee string) (err error) {
	var fields []string
	var sb strings.Builder

	userID, err := getOpUserIDByTxHash(config, message.TxHash)
	if err != nil {
		//admin deposit user id is 0
		if message.TxType != TYPE_ADMIN_DEPOSIT {
			return
		}
		userID = 0
	}
	flowID := generateFlowID(message, userID)

	fmt.Fprint(&sb, "(")
	fields = append(fields, fmt.Sprintf("%d", userID))
	fields = append(fields, strconv.Quote(flowID))
	fields = append(fields, strconv.Quote(fmt.Sprintf("%d", message.BlockTime)))
	fields = append(fields, strconv.Quote("资产充提"))

	coinID, ok := coinIds[symbol]
	if !ok {
		coinID, err = getCoinIDByName(config, symbol)
		if err == nil {
			coinIds[symbol] = coinID
		} else {
			err = errors.New("coin id not found")
			return
		}
	}

	fields = append(fields, strconv.Quote(fmt.Sprintf("1.3.1.%d", coinID)))
	fields = append(fields, strconv.Quote("管理员充币"))
	fields = append(fields, strconv.Quote(fmt.Sprintf("@%d@0", userID)))
	fields = append(fields, strconv.Quote(symbol))
	fields = append(fields, strconv.Quote(util.LeftShift(message.Amount.String(), 8)))
	fields = append(fields, strconv.Quote(message.TxHash))
	fields = append(fields, strconv.Quote(message.Address))
	fields = append(fields, strconv.Quote("SYS.A"))
	fields = append(fields, "1")
	fields = append(fields, "1")
	fields = append(fields, "1")

	fmt.Fprint(&sb, strings.Join(fields, ","))
	fmt.Fprint(&sb, ")")

	//for inner fund collection
	if message.TxType == TYPE_FUND_COLLECTION {
		sysFlowID := generateFlowID(message, userID)

		fields = fields[:0]
		fmt.Fprint(&sb, ", (")
		fields = append(fields, fmt.Sprintf("%d", userID))
		fields = append(fields, strconv.Quote(fmt.Sprintf("%s", sysFlowID)))
		fields = append(fields, fmt.Sprintf("%d", message.BlockTime))
		fields = append(fields, strconv.Quote("资产充提"))
		fields = append(fields, strconv.Quote(fmt.Sprintf("1.4.2.%d", coinID)))
		fields = append(fields, strconv.Quote("管理员提币链上手续费"))
		fields = append(fields, strconv.Quote(fmt.Sprintf("@%d@0", userID)))
		fields = append(fields, strconv.Quote(symbol))
		fields = append(fields, fee)
		fields = append(fields, strconv.Quote(message.TxHash))
		fields = append(fields, strconv.Quote("SYS.A"))
		fields = append(fields, strconv.Quote(message.Address))
		fields = append(fields, "1")
		fields = append(fields, "2")
		fields = append(fields, "1")

		fmt.Fprint(&sb, strings.Join(fields, ","))
		fmt.Fprint(&sb, ")")
	}

	status, err := saveFunFlow(config, sb.String())
	if err != nil {
		log.Println("save inner collection fund flow err:", status, err)
	}
	return
}

func saveAdminWithdrawTxFlow(config *conf.Config, message NotifyMessage, symbol, fee string) (err error) {
	var fields []string
	var sb strings.Builder
	userID, err := getOpUserIDByTxHash(config, message.TxHash)
	if err != nil {
		return
	}
	flowID := generateFlowID(message, userID)

	fmt.Fprint(&sb, "(")
	fields = append(fields, fmt.Sprintf("%d", userID))
	fields = append(fields, strconv.Quote(flowID))
	fields = append(fields, fmt.Sprintf("%d", message.BlockTime))
	fields = append(fields, strconv.Quote("资产充提"))

	coinID, ok := coinIds[symbol]
	if !ok {
		coinID, err = getCoinIDByName(config, symbol)
		if err == nil {
			coinIds[symbol] = coinID
		} else {
			err = errors.New("coin id not found")
			return
		}
	}

	fields = append(fields, strconv.Quote(fmt.Sprintf("1.4.1.%d", coinID)))
	fields = append(fields, strconv.Quote("管理员提币"))
	fields = append(fields, strconv.Quote(fmt.Sprintf("@%d@0", userID)))
	fields = append(fields, strconv.Quote(symbol))
	fields = append(fields, util.LeftShift(message.Amount.String(), 8))
	fields = append(fields, strconv.Quote(message.TxHash))
	fields = append(fields, strconv.Quote("SYS.A"))
	fields = append(fields, strconv.Quote(message.Address))
	fields = append(fields, "1")
	fields = append(fields, "2")
	fields = append(fields, "1")

	fmt.Fprint(&sb, strings.Join(fields, ","))
	fmt.Fprint(&sb, ")")
	sysFlowID := generateFlowID(message, userID)

	fields = fields[:0]
	fmt.Fprint(&sb, ", (")
	fields = append(fields, fmt.Sprintf("%d", userID))
	fields = append(fields, strconv.Quote(sysFlowID))
	fields = append(fields, fmt.Sprintf("%d", message.BlockTime))
	fields = append(fields, strconv.Quote("资产充提"))
	fields = append(fields, strconv.Quote(fmt.Sprintf("1.4.2.%d", coinID)))
	fields = append(fields, strconv.Quote("管理员提币链上手续费"))
	fields = append(fields, strconv.Quote(fmt.Sprintf("@%d@0", userID)))
	fields = append(fields, strconv.Quote(symbol))
	fields = append(fields, fee)
	fields = append(fields, strconv.Quote(message.TxHash))
	fields = append(fields, strconv.Quote("SYS.A"))
	fields = append(fields, strconv.Quote(message.Address))
	fields = append(fields, "1")
	fields = append(fields, "2")
	fields = append(fields, "1")

	fmt.Fprint(&sb, strings.Join(fields, ","))
	fmt.Fprint(&sb, ")")
	status, err := saveFunFlow(config, sb.String())
	if err != nil {
		log.Println("save admin withdraw fund flow err:", status, err)
	}
	return
}

func saveUserWithdrawTxFlow(config *conf.Config, message NotifyMessage, symbol, fee string) (err error) {
	var fields []string
	var sb strings.Builder
	userID, checker1, checker2, err := getUserIDsByTxHash(config, message.TxHash)
	flowID := generateFlowID(message, userID)

	fmt.Fprint(&sb, "(")
	fields = append(fields, fmt.Sprintf("%d", userID))
	fields = append(fields, strconv.Quote(flowID))
	fields = append(fields, fmt.Sprintf("%d", message.BlockTime))
	fields = append(fields, strconv.Quote("资产充提"))

	coinID, ok := coinIds[symbol]
	if !ok {
		coinID, err = getCoinIDByName(config, symbol)
		if err == nil {
			coinIds[symbol] = coinID
		} else {
			err = errors.New("coin id not found")
			return
		}
	}
	fields = append(fields, strconv.Quote(fmt.Sprintf("1.2.1.%d", coinID)))
	fields = append(fields, strconv.Quote("用户普通提币"))
	fields = append(fields, strconv.Quote(fmt.Sprintf("%d@%d@%d@0", userID, checker1, checker2)))
	fields = append(fields, strconv.Quote(fmt.Sprintf("%s", symbol)))
	fields = append(fields, util.LeftShift(message.Amount.String(), 8))
	fields = append(fields, strconv.Quote(message.TxHash))
	fields = append(fields, strconv.Quote(fmt.Sprintf("%d.0", userID)))
	fields = append(fields, strconv.Quote(message.Address))
	fields = append(fields, "1")
	fields = append(fields, "2")
	fields = append(fields, "1")

	fmt.Fprint(&sb, strings.Join(fields, ","))
	fmt.Fprint(&sb, ")")
	sysFlowID := generateFlowID(message, userID)

	fmt.Fprint(&sb, ", (")
	fields = fields[:0]

	fields = append(fields, fmt.Sprintf("%d", userID))
	fields = append(fields, strconv.Quote(sysFlowID))
	fields = append(fields, fmt.Sprintf("%d", message.BlockTime))
	fields = append(fields, strconv.Quote("资产充提"))
	fields = append(fields, strconv.Quote(fmt.Sprintf("1.2.3.%d", coinID)))
	fields = append(fields, strconv.Quote("用户提币链上手续费"))
	fields = append(fields, strconv.Quote(fmt.Sprintf("@%d@0", checker2)))
	fields = append(fields, strconv.Quote(fmt.Sprintf("%s", symbol)))
	fields = append(fields, fee)
	fields = append(fields, strconv.Quote(message.TxHash))
	fields = append(fields, strconv.Quote("SYS.A"))
	fields = append(fields, strconv.Quote(message.Address))
	fields = append(fields, "1")
	fields = append(fields, "2")
	fields = append(fields, "1")

	fmt.Fprint(&sb, strings.Join(fields, ","))
	fmt.Fprint(&sb, ")")
	status, err := saveFunFlow(config, sb.String())
	if err != nil {
		log.Println("save user withdraw fund flow err:", status, err)
	}
	return
}

func saveDepositTxFlow(config *conf.Config, message NotifyMessage, symbol, fee string) (err error) {
	var fields []string
	var sb strings.Builder
	userID, err := getUserIDByAddress(config, message.Address)
	if err != nil {
		return
	}
	flowID := generateFlowID(message, userID)

	fmt.Fprint(&sb, "(")
	fields = append(fields, fmt.Sprintf("%d", userID))
	fields = append(fields, strconv.Quote(flowID))
	fields = append(fields, fmt.Sprintf("%d", message.BlockTime))
	fields = append(fields, strconv.Quote("资产充提"))

	coinID, ok := coinIds[symbol]
	if !ok {
		coinID, err = getCoinIDByName(config, symbol)
		if err == nil {
			coinIds[symbol] = coinID
		} else {
			err = errors.New("coin id not found")
			return
		}
	}
	fields = append(fields, strconv.Quote(fmt.Sprintf("1.1.0.%d", coinID)))
	fields = append(fields, strconv.Quote("用户普通充币到钱包(到钱包)"))
	fields = append(fields, strconv.Quote(fmt.Sprintf("%d@0", userID)))
	fields = append(fields, strconv.Quote(symbol))
	fields = append(fields, util.LeftShift(message.Amount.String(), 8))
	fields = append(fields, strconv.Quote(message.TxHash))
	fields = append(fields, strconv.Quote(message.Address))
	fields = append(fields, strconv.Quote("SYS.A"))
	fields = append(fields, "1")
	fields = append(fields, "1")
	fields = append(fields, "1")

	fmt.Fprint(&sb, strings.Join(fields, ","))
	fmt.Fprint(&sb, ")")

	fields = fields[:0]
	fmt.Fprint(&sb, ", (")
	sysFlowID := generateFlowID(message, userID)

	fields = append(fields, fmt.Sprintf("%d", userID))
	fields = append(fields, strconv.Quote(sysFlowID))
	fields = append(fields, fmt.Sprintf("%d", message.BlockTime))
	fields = append(fields, strconv.Quote("资产充提"))
	fields = append(fields, strconv.Quote(fmt.Sprintf("1.1.1.%d", coinID)))
	fields = append(fields, strconv.Quote("用户普通充币到钱包(到系统)"))
	fields = append(fields, strconv.Quote(fmt.Sprintf("%d", userID)))
	fields = append(fields, strconv.Quote(symbol))
	fields = append(fields, util.LeftShift(message.Amount.String(), 8))
	fields = append(fields, strconv.Quote(flowID))
	fields = append(fields, strconv.Quote("SYS.A"))
	fields = append(fields, strconv.Quote(fmt.Sprintf("%d.0", userID)))
	fields = append(fields, "1")
	fields = append(fields, "1")
	fields = append(fields, "1")

	fmt.Fprint(&sb, strings.Join(fields, ","))
	fmt.Fprint(&sb, ")")

	status, err := saveFunFlow(config, sb.String())
	if err != nil {
		log.Println("save deposit fund flow err:", status, err)
	}
	return
}

func saveFunFlow(config *conf.Config, flowStr string) (status int, err error) {
	connStr := fmt.Sprintf("%s:%s@tcp(%s)/%s", config.DBUser, config.DBPass, config.DBHost, config.DBName)
	db, err := sql.Open("mysql", connStr)
	if err != nil {
		return
	}
	defer db.Close()

	rows, err := db.Query("call proc_sys_insFlowRecord(?)", flowStr)
	if err != nil {
		return
	}

	if rows.Next() {
		rows.Scan(&status)
	}
	if status != 1000 {
		err = errors.New(fmt.Sprintf("status is not 1000: %d", status))
	}
	log.Println(flowStr)
	return
}

func getCoinIDByName(config *conf.Config, coinName string) (coinID int, err error) {
	var status int
	connStr := fmt.Sprintf("%s:%s@tcp(%s)/%s", config.DBUser, config.DBPass, config.DBHost, config.DBName)
	db, err := sql.Open("mysql", connStr)
	if err != nil {
		return
	}
	defer db.Close()

	rows, err := db.Query("call proc_user_getCurrencyIdBySymbol(?)", coinName)
	if err != nil {
		return
	}

	if rows.Next() {
		rows.Scan(&coinID)
	}
	if rows.NextResultSet() {
		if rows.Next() {
			rows.Scan(&status)
		}
	}
	if status != 1000 {
		err = errors.New(fmt.Sprintf("status is not 1000: %d", status))
	}
	return
}

func getUserIDByAddress(config *conf.Config, address string) (userID int, err error) {
	var status int
	connStr := fmt.Sprintf("%s:%s@tcp(%s)/%s", config.DBUser, config.DBPass, config.DBHost, config.DBName)
	db, err := sql.Open("mysql", connStr)
	if err != nil {
		return
	}
	defer db.Close()

	rows, err := db.Query("call proc_user_getUserIdByRechargeAddress(?)", address)
	if err != nil {
		return
	}

	if rows.Next() {
		rows.Scan(&userID)
	}
	if rows.NextResultSet() {
		if rows.Next() {
			rows.Scan(&status)
		}
	}
	if status != 1000 {
		err = errors.New(fmt.Sprintf("status is not 1000: %d", status))
	}
	return
}

func getUserIDsByTxHash(config *conf.Config, txHash string) (userID, checker1, checker2 int, err error) {
	var status int
	connStr := fmt.Sprintf("%s:%s@tcp(%s)/%s", config.DBUser, config.DBPass, config.DBHost, config.DBName)
	db, err := sql.Open("mysql", connStr)
	if err != nil {
		return
	}
	defer db.Close()

	rows, err := db.Query("call proc_user_getAllUserIdByWithdrawHash(?)", txHash)
	if err != nil {
		return
	}

	if rows.Next() {
		rows.Scan(&userID, &checker1, &checker2)
	}
	if rows.NextResultSet() {
		if rows.Next() {
			rows.Scan(&status)
		}
	}
	if status != 1000 {
		err = errors.New(fmt.Sprintf("status is not 1000: %d", status))
	}
	return
}

func getOpUserIDByTxHash(config *conf.Config, txHash string) (userID int, err error) {
	var status int
	connStr := fmt.Sprintf("%s:%s@tcp(%s)/%s", config.DBUser, config.DBPass, config.DBHost, config.DBName)
	db, err := sql.Open("mysql", connStr)
	if err != nil {
		return
	}
	defer db.Close()

	rows, err := db.Query("call proc_user_getSysUserIdBySweepHash(?)", txHash)
	if err != nil {
		return
	}

	if rows.Next() {
		rows.Scan(&userID)
	}
	if rows.NextResultSet() {
		if rows.Next() {
			rows.Scan(&status)
		}
	}
	if status != 1000 {
		err = errors.New(fmt.Sprintf("status is not 1000: %d", status))
	}
	return
}

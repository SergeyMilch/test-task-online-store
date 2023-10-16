package main

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/jmoiron/sqlx"
	"github.com/joho/godotenv"
	"github.com/lib/pq"
	_ "github.com/lib/pq"
)

type StringArray []string

type OrderItem struct {
	OrderNumber       int         `db:"order_number"`
	ProductName       string      `db:"product_name"`
	ProductID         int         `db:"product_id"`
	Quantity          int         `db:"quantity"`
	ShelfName         string      `db:"shelf_name"`
	AdditionalShelves StringArray `db:"additional_shelves"`
}

type AdditionalShelvesResult struct {
	ProductID         int         `db:"product_id"`
	AdditionalShelves StringArray `db:"additional_shelves"`
}

func processOrders(db *sqlx.DB, orderNumbers []string) {

	// Подготавливаем запросы
	orderIDStmt, err := db.Preparex(`SELECT id FROM orders WHERE order_number = ANY($1)`)
	if err != nil {
		panic(err)
	}
	defer orderIDStmt.Close()

	productQueryStmt, err := db.Preparex(`
		SELECT 
			o.order_number,
			p.name AS product_name,
			p.id AS product_id,
			oi.quantity,
			s.name AS shelf_name
		FROM 
			order_items oi,
			orders o,
			products p,
			product_shelves ps,
			shelves s
		WHERE 
			o.id = oi.order_id AND
			p.id = oi.product_id AND
			ps.product_id = p.id AND
			s.id = ps.shelf_id AND
			ps.is_main = TRUE AND
			o.id = ANY($1)
	`)
	if err != nil {
		panic(err)
	}
	defer productQueryStmt.Close()

	additionalShelvesQueryStmt, err := db.Preparex(`
		SELECT 
			ps.product_id,
			array_agg(s.name ORDER BY s.name) AS additional_shelves
		FROM
			product_shelves ps,
			shelves s
		WHERE 
			s.id = ps.shelf_id AND
			ps.is_main = FALSE AND
			ps.product_id = ANY($1)
		GROUP BY
			ps.product_id
	`)
	if err != nil {
		panic(err)
	}
	defer additionalShelvesQueryStmt.Close()

	// Выполняем запросы
	var orderIDs []int
	err = orderIDStmt.Select(&orderIDs, pq.Array(orderNumbers))
	if err != nil {
		panic(err)
	}

	var orderItems []OrderItem
	err = productQueryStmt.Select(&orderItems, pq.Array(orderIDs))
	if err != nil {
		panic(err)
	}

	var additionalShelvesResults []AdditionalShelvesResult
	err = additionalShelvesQueryStmt.Select(&additionalShelvesResults, pq.Array(orderIDs))
	if err != nil {
		panic(err)
	}

	// Заполнение additionalShelvesMap
	additionalShelvesMap := make(map[int]StringArray)
	for _, result := range additionalShelvesResults {
		additionalShelvesMap[result.ProductID] = result.AdditionalShelves
	}

	// Обновление информации о дополнительных стеллажах в orderItems
	for i, item := range orderItems {
		orderItems[i].AdditionalShelves = additionalShelvesMap[item.ProductID]
	}

	// Вывод результатов
	var buffer bytes.Buffer

	buffer.WriteString("=+=+=+=\n")
	buffer.WriteString(fmt.Sprintf("Страница сборки заказов %s\n\n", strings.Join(orderNumbers, ",")))

	currentShelf := ""
	for _, item := range orderItems {
		if currentShelf != item.ShelfName {
			if currentShelf != "" {
				buffer.WriteString("\n")
			}
			buffer.WriteString(fmt.Sprintf("===Стеллаж %s", item.ShelfName))
			currentShelf = item.ShelfName
		}
		additionalShelfInfo := ""
		if len(item.AdditionalShelves) > 0 {
			additionalShelfInfo = fmt.Sprintf("\nдоп стеллаж: %s", strings.Join(item.AdditionalShelves, ","))
		}
		buffer.WriteString(fmt.Sprintf("\n%s (id=%d)\nзаказ %d, %d шт %s\n", item.ProductName, item.ProductID, item.OrderNumber, item.Quantity, additionalShelfInfo))
	}

	// Выводим буферизированный текст
	fmt.Println(buffer.String())
}

func (sa *StringArray) Scan(value interface{}) error {
	if value == nil {
		*sa = []string{}
		return nil
	}
	strArr := string(value.([]byte))
	*sa = strings.Split(strArr[1:len(strArr)-1], ",") // убираем скобки и разбиваем строку по запятой
	return nil
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Ошибка загрузки .env файла")
	}

	connStr := os.Getenv("DB_URL")
	db, err := sqlx.Connect("postgres", connStr)
	if err != nil {
		panic(err)
	}

	orderNumbers := strings.Split(os.Args[1], ",")
	processOrders(db, orderNumbers)
}

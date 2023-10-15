package main

import (
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

func processOrders(db *sqlx.DB, orderNumbers []string) {
	query := `
    WITH order_data AS (
        SELECT 
            o.order_number,
            p.name AS product_name,
            p.id AS product_id,  -- здесь явно указываем, что имеем в виду p.id
            oi.quantity,
            s.name AS shelf_name,
            ps.is_main
        FROM 
            order_items oi
        JOIN 
            orders o ON o.id = oi.order_id
        JOIN 
            products p ON p.id = oi.product_id
        JOIN 
            product_shelves ps ON ps.product_id = p.id  -- и здесь тоже явно указываем, что имеем в виду p.id
        JOIN 
            shelves s ON s.id = ps.shelf_id
        WHERE 
            o.order_number = ANY($1)
        AND 
            ps.is_main = TRUE
    )
    SELECT
        order_number,
        product_name,
        od.product_id,  -- и здесь тоже явно указываем, что имеем в виду od.product_id
        quantity,
        shelf_name,
        array_agg(s.name ORDER BY s.name) AS additional_shelves
    FROM
        order_data od
    LEFT JOIN 
        product_shelves ps ON ps.product_id = od.product_id  -- и здесь тоже явно указываем, что имеем в виду od.product_id
    LEFT JOIN
        shelves s ON s.id = ps.shelf_id
    GROUP BY
        order_number, product_name, od.product_id, quantity, shelf_name  -- и здесь тоже явно указываем, что имеем в виду od.product_id
    ORDER BY 
        shelf_name, product_name, order_number;
`

	var orderItems []OrderItem
	err := db.Select(&orderItems, query, pq.Array(orderNumbers))
	if err != nil {
		panic(err)
	}

	fmt.Println("=+=+=+=")
	fmt.Printf("Страница сборки заказов %s\n\n", strings.Join(orderNumbers, ","))

	currentShelf := ""
	for _, item := range orderItems {
		if currentShelf != item.ShelfName {
			if currentShelf != "" {
				fmt.Println()
			}
			fmt.Printf("===Стеллаж %s", item.ShelfName)
			currentShelf = item.ShelfName
		}
		additionalShelfInfo := ""
		if len(item.AdditionalShelves) > 0 {
			additionalShelfInfo = fmt.Sprintf("\nдоп стеллаж: %s", strings.Join(item.AdditionalShelves, ","))
		}
		fmt.Printf("\n%s (id=%d)\nзаказ %d, %d шт %s\n", item.ProductName, item.ProductID, item.OrderNumber, item.Quantity, additionalShelfInfo)
	}
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

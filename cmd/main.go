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

// Структура элементов заказа
type OrderItem struct {
	OrderNumber int    `db:"order_number"`
	ProductName string `db:"product_name"`
	ProductID   int    `db:"product_id"`
	Quantity    int    `db:"quantity"`
	ShelfName   string `db:"shelf_name"`
}

// Структура данных о стеллажах
type ShelfData struct {
	ProductID int    `db:"product_id"`
	IsMain    bool   `db:"is_main"`
	ShelfName string `db:"name"`
}

// Структура информации о стеллажах
type ShelfInfo struct {
	ShelfName         string
	AdditionalShelves []string
}

// Функция для обработки заказов
func processOrders(db *sqlx.DB, orderNumbers []string) {

	// Запрос для получения ID заказов по номерам заказов
	orderIDQuery := `SELECT id, order_number FROM orders WHERE order_number = ANY($1)`
	var orderIDs []struct {
		ID          int `db:"id"`
		OrderNumber int `db:"order_number"`
	}
	// Выполнение запроса и сохранение результатов в orderIDs
	err := db.Select(&orderIDs, orderIDQuery, pq.Array(orderNumbers))
	if err != nil {
		panic(err)
	}

	// Создание списка идентификаторов заказов
	var orderIDList []int
	orderNumberMap := make(map[int]int) // карта для сопоставления order_id с order_number
	for _, orderID := range orderIDs {
		orderIDList = append(orderIDList, orderID.ID)
		orderNumberMap[orderID.ID] = orderID.OrderNumber
	}

	// Получение элементов заказов
	itemQuery := `SELECT order_id, product_id, quantity FROM order_items WHERE order_id = ANY($1)`
	var orderItems []struct {
		OrderID   int `db:"order_id"`
		ProductID int `db:"product_id"`
		Quantity  int `db:"quantity"`
	}
	err = db.Select(&orderItems, itemQuery, pq.Array(orderIDList))
	if err != nil {
		panic(err)
	}

	// Выбираем product_id только те, которые в заказе
	var uniqueProductIDs []int
	productIDSet := make(map[int]struct{}) // Используется для устранения дубликатов
	for _, item := range orderItems {
		if _, exists := productIDSet[item.ProductID]; !exists {
			uniqueProductIDs = append(uniqueProductIDs, item.ProductID)
			productIDSet[item.ProductID] = struct{}{}
		}
	}

	// Получение информации о товарах
	productQuery := `SELECT id, name FROM products WHERE id = ANY($1)`
	var products []struct {
		ID   int    `db:"id"`
		Name string `db:"name"`
	}
	err = db.Select(&products, productQuery, pq.Array(uniqueProductIDs))
	if err != nil {
		panic(err)
	}

	// Структурирование данных
	productMap := make(map[int]string)
	for _, product := range products {
		productMap[product.ID] = product.Name
	}

	// Получение данных о связи товаров и стеллажей
	shelfQuery := `
    SELECT 
        ps.product_id,
        ps.is_main,
        s.name
    FROM 
        product_shelves ps
    JOIN 
        shelves s ON ps.shelf_id = s.id;
	WHERE 
        ps.product_id = ANY($1);
	`

	// Выполнение запроса и сохранение результатов в shelves
	var shelves []ShelfData
	err = db.Select(&shelves, shelfQuery, pq.Array(uniqueProductIDs))
	if err != nil {
		panic(err)
	}

	// Создание карты для хранения информации о стеллажах для каждого продукта
	shelfInfoMap := make(map[int]ShelfInfo)
	additionalShelvesMap := make(map[int][]string) // карта для хранения дополнительных стеллажей

	for _, shelf := range shelves {
		if shelf.IsMain {
			// Если это основной стеллаж, сохраняем его имя в карту
			shelfInfoMap[shelf.ProductID] = ShelfInfo{ShelfName: shelf.ShelfName}
		} else {
			// Если это дополнительный стеллаж, добавляем его имя в список дополнительных стеллажей
			additionalShelvesMap[shelf.ProductID] = append(additionalShelvesMap[shelf.ProductID], shelf.ShelfName)
		}
	}

	// Объединение информации о дополнительных стеллажах с основной информацией о стеллажах
	for productID, additionalShelves := range additionalShelvesMap {
		shelfInfo := shelfInfoMap[productID]
		shelfInfo.AdditionalShelves = additionalShelves
		shelfInfoMap[productID] = shelfInfo
	}

	// Вывод результата
	fmt.Println("=+=+=+=")
	fmt.Printf("Страница сборки заказов %s\n\n", strings.Join(orderNumbers, ","))

	currentShelf := ""
	for _, item := range orderItems {
		shelfInfo := shelfInfoMap[item.ProductID]
		shelfName := shelfInfo.ShelfName
		if currentShelf != shelfName {
			if currentShelf != "" {
				fmt.Println()
			}
			fmt.Printf("===Стеллаж %s", shelfName)
			currentShelf = shelfName
		}
		additionalShelfInfo := ""
		if len(shelfInfo.AdditionalShelves) > 0 {
			additionalShelfInfo = fmt.Sprintf("\nдоп стеллаж: %s", strings.Join(shelfInfo.AdditionalShelves, ","))
		}
		fmt.Printf("\n%s (id=%d)\nзаказ %d, %d шт%s\n", productMap[item.ProductID], item.ProductID, orderNumberMap[item.OrderID], item.Quantity, additionalShelfInfo)
	}
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

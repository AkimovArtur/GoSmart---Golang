package main

import ( //Импорты для работы приложения 
	"database/sql"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gorilla/mux"
	_ "github.com/lib/pq" // Подключаем драйвер PostgreSQL
	"golang.org/x/crypto/bcrypt"
	"golangify.com/snippetbox/products"
	"github.com/gorilla/sessions"
)

// Шаблоны для переменных и для html страниц
var marketTpl = template.Must(template.ParseFiles("index/market.html"))
var aboutTpl = template.Must(template.ParseFiles("index/about.html"))
var productTpl = template.Must(template.ParseFiles("index/product.html"))
var productMobileTpl = template.Must(template.ParseFiles("index/productMobile.html"))
var registrationTpl = template.Must(template.ParseFiles("index/registration.html"))
var loginTpl = template.Must(template.ParseFiles("index/login.html")) // Шаблон для входа
var adminTpl = template.Must(template.ParseFiles("index/add_product.html")) // Шаблон для админкий


// Подключение к базе данных
var db *sql.DB
var store = sessions.NewCookieStore([]byte("секретный_ключ")) // Замените на ваш секретный ключ

func initDB() {
	var err error
	connStr := "user=postgres password=postgresql dbname=myapp sslmode=disable" // Подключение к нашей БД
	db, err = sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal(err)
	}
}

func loadProducts() {
	// Загрузка обычных продуктов
	rows, err := db.Query("SELECT id, name, description, image_url, price, processor, ram, drive, display FROM products") //запросом, запрашивает колон и инфу
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	for rows.Next() {
		var product products.Product //загрузка информации
		if err := rows.Scan(&product.ID, &product.Name, &product.Description, &product.ImageURL, &product.Price, &product.Processor, &product.RAM, &product.Drive, &product.Display); err != nil {
			log.Fatal(err)
		}

		// Заменяем обратные слэши на прямые
		product.ImageURL = strings.Replace(product.ImageURL, "\\", "/", -1)

		// Добавляем продукт в карту
		products.Products[product.ID] = product
	}

	// Загрузка мобильных продуктов
	rowsMobile, err := db.Query("SELECT id, name, description, image_url, price, display, camera, drive, color FROM products_mobile")
	if err != nil {
		log.Fatal(err)
	}
	defer rowsMobile.Close()

	for rowsMobile.Next() {
		var mobileProduct products.ProductMobile
		if err := rowsMobile.Scan(&mobileProduct.ID1, &mobileProduct.Name1, &mobileProduct.Description1, &mobileProduct.ImageURL1, &mobileProduct.Price1, &mobileProduct.Display1, &mobileProduct.Camera, &mobileProduct.Drive1, &mobileProduct.Color); err != nil {
			log.Fatal(err)
		}

		// Заменяем обратные слэши на прямые
		mobileProduct.ImageURL1 = strings.Replace(mobileProduct.ImageURL1, "\\", "/", -1)

		// Добавляем мобильный продукт в карту
		products.ProductsMobile[mobileProduct.ID1] = mobileProduct
	}
}


// Обработчик для страницы товара
func productHandler(w http.ResponseWriter, r *http.Request) {
	loadProducts() //загрузка товаров 
    vars := mux.Vars(r)

	productID := vars["id"] 
	product, exists := products.Products[productID] //извлечение ID продукта
	if !exists {
		http.NotFound(w, r)
		return
	}

	err := productTpl.Execute(w, product) // Передаем объект product в шаблон html со странцей товара
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// Обработчик для мобильного товара
func productHandlerMobile(w http.ResponseWriter, r *http.Request) {
	loadProducts()
    vars := mux.Vars(r)
	productID := vars["id"]

	product, exists := products.ProductsMobile[productID]
	if !exists {
		http.NotFound(w, r)
		return
	}

	err := productMobileTpl.Execute(w, product) // Передаем объект product в шаблон
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Println("Ошибка при рендеринге шаблона:", err) // Логирование ошибки
	}
}

// Обработчик для главной страницы


// Обработчик для отображения формы добавления товара
func addProductHandler(w http.ResponseWriter, r *http.Request) { //функция для обработки страницы добавления товара (используется шаблон)
	adminTpl.Execute(w, nil) // Отображение формы добавления товара
}

// Обработчик для добавления обычного товара
func addProduct(w http.ResponseWriter, r *http.Request) { //функця для добавления товара
    if r.Method == http.MethodPost { //считывания информации по товару из формы
        name := r.FormValue("name")
        description := r.FormValue("description")
        priceStr := r.FormValue("price")
        processor := r.FormValue("processor") // Считываем процессор
        ram := r.FormValue("ram")             // Считываем оперативную память
        drive := r.FormValue("drive")         // Считываем накопитель
        display := r.FormValue("display")     // Считываем дисплей

        price, err := strconv.ParseFloat(priceStr, 64) //strconv конвертирует строку в float64
        if err != nil {
            log.Println("Ошибка при преобразовании цены:", err) //выводилось в консоль сообщение об ошибке 
            http.Error(w, "Ошибка при преобразовании цены", http.StatusBadRequest) //выводилось на странице сообщение об ошибке 
            return
        }

        file, header, err := r.FormFile("image")
        if err != nil {
            log.Println("Ошибка при получении файла:", err)
            http.Error(w, "Ошибка при получении файла", http.StatusBadRequest)
            return
        }
        defer file.Close()

        // Создаем директорию для изображений, если она не существует
        imageDir := filepath.Join("assets", "product_images")
        if err := os.MkdirAll(imageDir, os.ModePerm); err != nil { //условие для создания папки, добвления фотки в неё
            log.Println("Ошибка при создании директории:", err)
            http.Error(w, "Ошибка при создании директории", http.StatusInternalServerError)
            return
        }

        // Путь к изображению
        imagePath := filepath.Join(imageDir, header.Filename)

        // Создаем файл для изображения
        out, err := os.Create(imagePath)
        if err != nil {
            log.Println("Ошибка при сохранении файла:", err)
            http.Error(w, "Ошибка при сохранении файла", http.StatusInternalServerError) //условия для добавления фоток
            return
        }
        defer out.Close()

        // Копируем содержимое загруженного файла в созданный файл
        if _, err := io.Copy(out, file); err != nil {
            log.Println("Ошибка при записи файла:", err)
            http.Error(w, "Ошибка при записи файла", http.StatusInternalServerError)
            return
        }

        // Заменяем обратные слэши на прямые перед сохранением в базу данных
        imagePath = strings.Replace(imagePath, "\\", "/", -1)

        var id int
        err = db.QueryRow("INSERT INTO products (name, description, price, image_url, processor, ram, drive, display) VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING id",
            name, description, price, imagePath, processor, ram, drive, display).Scan(&id)

        if err != nil {
            log.Println("Ошибка при добавлении товара в базу данных:", err)
            http.Error(w, "Ошибка при добавлении товара", http.StatusInternalServerError)
            return
        }

        products.Products[fmt.Sprint(id)] = products.Product{
            ID:          fmt.Sprint(id),
            Name:        name,
            Description: description,
            Price:       price,
            ImageURL:    imagePath,
            Processor:   processor,
            RAM:         ram,
            Drive:       drive,
            Display:     display,
        }

        log.Println("Товар успешно добавлен:", name)

        http.Redirect(w,r,"/admin",http.StatusSeeOther)
    }
}

// Обработчик для добавления мобильного товара
func addMobileProduct(w http.ResponseWriter, r *http.Request) {
    if r.Method == http.MethodPost {
        name := r.FormValue("name")
        description := r.FormValue("description")
        priceStr := r.FormValue("price") // Считываем цену как строку

        // Преобразуем строку в float64 для цены.
        price, err := strconv.ParseFloat(priceStr, 64)
        if err != nil {
            log.Println("Ошибка при преобразовании цены:", err)
            http.Error(w, "Ошибка при преобразовании цены", http.StatusBadRequest)
            return
        }

        file, header, err := r.FormFile("image")
        if err != nil {
            log.Println("Ошибка при получении файла:", err) // Логируем ошибку.
            http.Error(w, "Ошибка при получении файла", http.StatusBadRequest)
            return
        }
        defer file.Close()

        // Создаем директорию для изображений, если она не существует
        imageDir := filepath.Join("assets", "product_images")
        if err := os.MkdirAll(imageDir, os.ModePerm); err != nil {
            log.Println("Ошибка при создании директории:", err) // Логируем ошибку.
            http.Error(w, "Ошибка при создании директории", http.StatusInternalServerError)
            return
        }

        // Путь к изображению
        imagePath := filepath.Join(imageDir, header.Filename)

        // Создаем файл для изображения
        out, err := os.Create(imagePath)
        if err != nil {
            log.Println("Ошибка при сохранении файла:", err) // Логируем ошибку.
            http.Error(w, "Ошибка при сохранении файла", http.StatusInternalServerError)
            return
        }
        defer out.Close()

        // Копируем содержимое загруженного файла в созданный файл
        if _, err := io.Copy(out, file); err != nil {
            log.Println("Ошибка при записи файла:", err) // Логируем ошибку.
            http.Error(w, "Ошибка при записи файла", http.StatusInternalServerError)
            return
        }

        // Заменяем обратные слэши на прямые перед сохранением в базу данных
        imagePath = strings.Replace(imagePath, "\\", "/", -1)

        display := r.FormValue("display")
        camera := r.FormValue("camera")
        drive := r.FormValue("drive")
        color := r.FormValue("color")

        _, err = db.Exec("INSERT INTO products_mobile (name, description, price, image_url, display, camera, drive, color) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)",
            name, description, price, imagePath, display, camera, drive, color)

        if err != nil {
            log.Println("Ошибка при добавлении мобильного товара в базу данных:", err) // Логируем ошибку.
            http.Error(w, "Ошибка при добавлении мобильного товара", http.StatusInternalServerError)
            return
        }

        products.ProductsMobile[fmt.Sprint(len(products.ProductsMobile)+1)] = products.ProductMobile{
            ID1:          fmt.Sprint(len(products.ProductsMobile) + 1),
            Name1:        name,
            Description1: description,
            Price1:       price,
            ImageURL1:    imagePath,
            Display1:     display,
            Camera:       camera,
            Drive1:       drive,
            Color:        color,
        }

        log.Println("Мобильный товар успешно добавлен:", name)

        http.Redirect(w, r, "/admin", http.StatusSeeOther)
    }
}


// Обработчик для удаления товара
func deleteProduct(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	productID := vars["id"]

    // Удаление из базы данных обычного товара.
    _, err := db.Exec("DELETE FROM products WHERE id = $1", productID) //запрос на удаление в БД

    if err != nil {
        log.Println("Ошибка при удалении товара из базы данных:", err)
        http.Error(w,"Ошибка при удалении товара",http.StatusInternalServerError)
        return 
    }

    delete(products.Products, productID) // Удаление из карты

    log.Println("Товар успешно удален:", productID)

    http.Redirect(w,r,"/admin",http.StatusSeeOther) // Перенаправление на страницу админки.
}

// Обработчик для удаления мобильного товара
func deleteMobileProduct(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	productID := vars["id"]

	// Удаление из базы данных мобильного товара.
	_, err := db.Exec("DELETE FROM products_mobile WHERE id = $1", productID)

	if err != nil {
		log.Println("Ошибка при удалении мобильного товара из базы данных:", err)
		http.Error(w,"Ошибка при удалении товара",http.StatusInternalServerError)
		return 
	}

	delete(products.ProductsMobile, productID) // Удаление из карты мобильных товаров.

	log.Println("Мобильный товар успешно удален:", productID)

	http.Redirect(w,r,"/admin",http.StatusSeeOther) // Перенаправление на страницу админки.
}


func orderHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	productID := vars["id"]
	
	// Определяем тип продукта (обычный или мобильный)
	product, exists := products.Products[productID]
	if !exists {
		productMobile, existsMobile := products.ProductsMobile[productID]
		if !existsMobile {
			http.NotFound(w, r)
			return
		}
		// Отображаем форму для мобильного продукта
		err := template.Must(template.ParseFiles("index/order.html")).Execute(w, map[string]interface{}{
			"ProductName": productMobile.Name1,
			"IsMobile":    true,
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	// Отображаем форму для обычного продукта
	err := template.Must(template.ParseFiles("index/order.html")).Execute(w, map[string]interface{}{
		"ProductName": product.Name,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}


func submitOrderHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		productName := r.FormValue("product_name")
		firstName := r.FormValue("first_name")
		lastName := r.FormValue("last_name")
		middleName := r.FormValue("middle_name")
		phone := r.FormValue("phone")
		quantityStr := r.FormValue("quantity")
		region := r.FormValue("region")
		city := r.FormValue("city")
		street := r.FormValue("street")
		house := r.FormValue("house")
		apartment := r.FormValue("apartment")

        quantity, err := strconv.Atoi(quantityStr)
        if err != nil {
            log.Println("Ошибка при преобразовании количества:", err)
            http.Error(w, "Ошибка при преобразовании количества", http.StatusBadRequest)
            return
        }

        // Сохранение данных в базу данных
        _, err = db.Exec("INSERT INTO orders (product_name, first_name, last_name, middle_name, phone, quantity, region, city, street, house, apartment) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)",
            productName, firstName, lastName, middleName, phone, quantity, region, city, street, house, apartment)

        if err != nil {
            log.Println("Ошибка при сохранении заказа в базу данных:", err)
            http.Error(w,"Ошибка при сохранении заказа",http.StatusInternalServerError)
            return 
        }

        log.Println("Заказ успешно оформлен:", productName)

        // Перенаправление на страницу подтверждения или главную страницу
        http.Redirect(w,r,"index/order_success.html",http.StatusSeeOther)
    }
}


// Остальные обработчики...
func marketHandler(w http.ResponseWriter,r *http.Request) { //Отображения пользователя
    loadProducts() // Загружаем продукты перед отображением страницы

    data := struct{
        Products      map[string]products.Product
        ProductsMobile map[string]products.ProductMobile
        Username      string
        Role          string
    }{
        Products:      products.Products,
        ProductsMobile: products.ProductsMobile, //Доделать
    }

    session, _ := store.Get(r,"session-name")
    if username, ok := session.Values["username"].(string); ok {
        data.Username = username
    }
	
    if role, ok := session.Values["role"].(string); ok {
        data.Role = role
    }

    if err := marketTpl.Execute(w,data); err != nil {
        http.Error(w,"Ошибка рендеринга страницы",http.StatusInternalServerError)
    }
}

func aboutHandler(w http.ResponseWriter,r *http.Request){ //функция для отображения шаблона О нас
    aboutTpl.Execute(w,nil)
}

//func cartHandler(w http.ResponseWriter,r *http.Request){
    //cartTpl.Execute(w,nil)
//}

func registrationHandler(w http.ResponseWriter,r *http.Request){ //функция для регистрации
    if r.Method==http.MethodPost{
        username:=r.FormValue("username")
        password:=r.FormValue("password")
        email:=r.FormValue("email")

        hashedPassword ,err:=bcrypt.GenerateFromPassword([]byte(password),bcrypt.DefaultCost)

        if err!=nil{
            http.Error(w,"Ошибка хеширования пароля",http.StatusInternalServerError)
            return 
        }

        role:="user" // По умолчанию роль пользователя

        if username=="admin"{ // Назначаем роль администратору по имени пользователя 
            role="admin"
        }

        _,err=db.Exec("INSERT INTO users (username,password,email ,role) VALUES ($1,$2,$3,$4)",username ,hashedPassword,email ,role)

        if err!=nil{
            log.Printf("Ошибка при сохранении пользователя:%v",err)
            http.Error(w,"Ошибка при сохранении пользователя",http.StatusInternalServerError)
            return 
        }

        http.Redirect(w, r, "/", http.StatusSeeOther) // Перенаправление на страницу товаров после успешной регистрации или входа.
       return 
    }
    registrationTpl.Execute(w,nil) // Отображение формы регистрации 
}

// Обработчик для входа в систему 
func loginHandler(w http.ResponseWriter,r *http.Request){ //функция для входа
	if r.Method==http.MethodPost{ 
    	username:=r.FormValue("username") 
    	password:=r.FormValue("password") 

    	var hashedPassword string 
    	var role string 
    	err:=db.QueryRow("SELECT password ,role FROM users WHERE username=$1",username).Scan(&hashedPassword,&role)

    	if err!=nil||bcrypt.CompareHashAndPassword([]byte(hashedPassword),[]byte(password))!=nil{ 
        	http.Error(w,"Неверное имя пользователя или пароль",http.StatusUnauthorized) 
        	return 
    	} 

    	session,_:=store.Get(r,"session-name") 
    	session.Values["username"]=username 
    	session.Values["role"]=role // Сохраняем роль в сессии 
    	session.Save(r,w)

    	http.Redirect(w, r, "/", http.StatusSeeOther) // Перенаправление на страницу товаров после успешной регистрации или входа. 
    	return 
    }

	loginTpl.Execute(w,nil) // Отображение формы входа 
}

// Обработчик для выхода из системы 
func logoutHandler(w http.ResponseWriter,r *http.Request){  //функция для выхода
	session,_:=store.Get(r,"session-name") 

	delete(session.Values,"username") 
	delete(session.Values,"role") 
	session.Save(r,w)

	http.Redirect(w,r,"/",http.StatusSeeOther) 
}

// Обработчик для админки 
func adminHandler(w http.ResponseWriter,r *http.Request){ 
	session,_:=store.Get(r,"session-name")

	if session.Values["role"]!="admin"{ 
	    http.Error(w,"Доступ запрещен",http.StatusForbidden) 
	    return 
    }

	data := struct{
	   Products      map[string]products.Product  // Добавляем мапу продуктов.
	   ProductsMobile map[string]products.ProductMobile  // Добавляем мапу мобильных продуктов.
   }{
	   Products:      products.Products,
	   ProductsMobile: products.ProductsMobile,
   }

	adminTpl.Execute(w,data)// Отображение страницы админки с данными о продуктах.
}

func main() { 
	initDB() // Инициализация базы данных 
	loadProducts() // Загрузка товаров из базы данных

	r := mux.NewRouter() 

    

	fs := http.FileServer(http.Dir("assets")) //показываем папку с ассетами
	r.PathPrefix("/assets/").Handler(http.StripPrefix("/assets/", fs))

    fsHTML := http.FileServer(http.Dir("index"))
    r.PathPrefix("/index/").Handler(http.StripPrefix("/index/", fsHTML))

	r.HandleFunc("/", marketHandler).Methods("GET")
	r.HandleFunc("/about", aboutHandler).Methods("GET")      // Обработчик для страницы "О нас" 
	//r.HandleFunc("/cart", cartHandler).Methods("GET")        // Обработчик для корзины 
	
	r.HandleFunc("/registration", registrationHandler).Methods("GET", "POST") // Обработчик для регистрации  
    r.HandleFunc("/login", loginHandler).Methods("GET", "POST")               // Обработчик для входа  
    r.HandleFunc("/logout", logoutHandler).Methods("GET")                     // Обработчик для выхода       
	r.HandleFunc("/admin", adminHandler).Methods("GET")                       // Обработчик для админки 

	r.HandleFunc("/product/{id:[0-9]+}", productHandler).Methods("GET")          // Обработчик для страницы товара  
	r.HandleFunc("/mobile/{id:[0-9]+}", productHandlerMobile).Methods("GET")    // Обработчик для мобильного 

	r.HandleFunc("/admin/add-product", addProduct).Methods("POST")               // Обработчик для добавления товара.
	r.HandleFunc("/admin/add-mobile-product", addMobileProduct).Methods("POST")  // Обработчик для добавления мобильного товара.

	r.HandleFunc("/admin/delete-product/{id:[0-9]+}", deleteProduct).Methods("POST")  // Обработчик для удаления товара.
	r.HandleFunc("/admin/delete-mobile-product/{id:[0-9]+}", deleteMobileProduct).Methods("POST")  // Обработчик для удаления мобильного товара.

    r.HandleFunc("/order/product/{id:[0-9]+}", orderHandler).Methods("GET")
    r.HandleFunc("/order/mobile/{id:[0-9]+}", orderHandler).Methods("GET")
    r.HandleFunc("/order", submitOrderHandler).Methods("POST")


	log.Println("Запуск веб-сервера на http://localhost:9090")
	err := http.ListenAndServe(":9090", r) // Используем маршрутизатор gorilla/mux  
	log.Fatal(err) 
}

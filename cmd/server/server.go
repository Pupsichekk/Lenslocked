package main

import (
	"fmt"
	"net/http"
	"os"
	"strconv"

	"github.com/Pupsichekk/lenslocked/controllers"
	"github.com/Pupsichekk/lenslocked/migrations"
	"github.com/Pupsichekk/lenslocked/models"
	"github.com/Pupsichekk/lenslocked/templates"
	"github.com/Pupsichekk/lenslocked/views"
	"github.com/go-chi/chi/v5"
	"github.com/gorilla/csrf"
	"github.com/joho/godotenv"
)

type config struct {
	PSQL models.PostgresConfig
	SMTP models.SMTPConfig
	CSRF struct {
		Key    string
		Secure bool
	}
	Server struct {
		Address string
	}
}

func loadEnvConfig() (config, error) {
	var cfg config
	if err := godotenv.Load(); err != nil {
		return cfg, nil
	}

	cfg.PSQL = models.PostgresConfig{
		Host:     os.Getenv("PSQL_HOST"),
		Port:     os.Getenv("PSQL_PORT"),
		User:     os.Getenv("PSQL_USER"),
		Password: os.Getenv("PSQL_PASSWORD"),
		Database: os.Getenv("PSQL_DATABASE"),
		Sslmode:  os.Getenv("PSQL_SSLMODE"),
	}

	if cfg.PSQL.Port == "" || cfg.PSQL.Host == "" {
		return cfg, fmt.Errorf("NO PSQL config provided")
	}

	cfg.SMTP.Host = os.Getenv("SMTP_HOST")
	portStr := os.Getenv("SMTP_PORT")
	port, err := strconv.Atoi(portStr)
	if err != nil {
		panic(err)
	}
	cfg.SMTP.Port = port
	cfg.SMTP.Username = os.Getenv("SMTP_USERNAME")
	cfg.SMTP.Password = os.Getenv("SMTP_PASSWORD")
	// TODO: CSRF
	cfg.CSRF.Key = os.Getenv("CSRF_KEY")
	cfg.CSRF.Secure = os.Getenv("CSRF_SECURE") == "true"
	// TODO: Read The Server Files from an .env var
	cfg.Server.Address = os.Getenv("SERVER_ADDRESS")
	return cfg, nil
}

func main() {
	cfg, err := loadEnvConfig()
	if err != nil {
		panic(err)
	}
	db, err := models.Open(cfg.PSQL)
	if err != nil {
		panic(err)
	}
	defer db.Close()

	err = models.MigrateFS(db, migrations.FS, ".")
	if err != nil {
		panic(err)
	}

	// Setup services
	userService := &models.UserService{
		DB: db,
	}
	sessionService := &models.SessionService{
		DB: db,
	}
	pwResetService := &models.PasswordResetService{
		DB: db,
	}
	emailService := models.NewEmailService(cfg.SMTP)
	galleryService := &models.GalleryService{
		DB: db,
	}

	// Setup middleware
	umw := controllers.UserMiddleware{
		SessionService: sessionService,
	}

	csrfMw := csrf.Protect(
		[]byte(cfg.CSRF.Key),
		csrf.Secure(cfg.CSRF.Secure),
		csrf.Path("/"),
	)

	// Setup controllers
	usersC := controllers.Users{
		UserService:          userService,
		SessionService:       sessionService,
		PasswordResetService: pwResetService,
		EmailService:         emailService,
		GalleryService:       galleryService,
	}
	usersC.Templates.New = views.Must(views.ParseFS(templates.FS, "signup.gohtml", "tailwind.gohtml"))
	usersC.Templates.SignIn = views.Must(views.ParseFS(templates.FS, "signin.gohtml", "tailwind.gohtml"))
	usersC.Templates.ForgotPassword = views.Must(views.ParseFS(templates.FS, "forgot-pw.gohtml", "tailwind.gohtml"))
	usersC.Templates.CheckYourEmail = views.Must(views.ParseFS(templates.FS,
		"check-your-email.gohtml", "tailwind.gohtml"))
	usersC.Templates.ResetPassword = views.Must(views.ParseFS(templates.FS,
		"reset-pw.gohtml", "tailwind.gohtml"))
	galleriesC := controllers.Galleries{
		GalleryService: galleryService,
	}
	galleriesC.Templates.New = views.Must(views.ParseFS(templates.FS,
		"galleries/new.gohtml", "tailwind.gohtml"))
	galleriesC.Templates.Edit = views.Must(views.ParseFS(templates.FS,
		"galleries/edit.gohtml", "tailwind.gohtml"))
	galleriesC.Templates.Index = views.Must(views.ParseFS(templates.FS,
		"galleries/index.gohtml", "tailwind.gohtml"))
	galleriesC.Templates.Show = views.Must(views.ParseFS(templates.FS,
		"galleries/show.gohtml", "tailwind.gohtml"))

	// Setup router and routes
	r := chi.NewRouter()
	r.Use(csrfMw)
	r.Use(umw.SetUser)
	tpl := views.Must(views.ParseFS(templates.FS, "home.gohtml", "tailwind.gohtml"))
	r.Get("/", controllers.StaticHandler(tpl))
	tpl = views.Must(views.ParseFS(templates.FS, "contact.gohtml", "tailwind.gohtml"))
	r.Get("/contact", controllers.StaticHandler(tpl))
	tpl = views.Must(views.ParseFS(templates.FS, "faq.gohtml", "tailwind.gohtml"))
	r.Get("/faq", controllers.FAQ(tpl))
	r.Get("/signup", usersC.New)
	r.Post("/users", usersC.Create)
	r.Get("/signin", usersC.SignIn)
	r.Post("/signin", usersC.ProcessSignIn)
	r.Post("/signout", usersC.ProcessSignOut)
	r.Get("/forgot-pw", usersC.ForgotPassword)
	r.Post("/forgot-pw", usersC.ProcessForgotPassword)
	r.Get("/reset-pw", usersC.ResetPassword)
	r.Post("/reset-pw", usersC.ProcessResetPassword)
	r.Route("/users/me", func(r chi.Router) {
		r.Use(umw.RequireUser)
		r.Get("/", usersC.CurrentUser)
	})
	r.Route("/galleries", func(r chi.Router) {
		r.Get("/{id}", galleriesC.Show)
		r.Get("/{id}/images/{filename}", galleriesC.Image)
		// http forms are whacky, have to use post, otherwise would've used normal methods
		r.Group(func(r chi.Router) {
			r.Use(umw.RequireUser)
			r.Get("/", galleriesC.Index)
			r.Get("/new", galleriesC.New)
			r.Post("/", galleriesC.Create)
			r.Get("/{id}/edit", galleriesC.Edit)
			r.Post("/{id}", galleriesC.Update)
			r.Post("/{id}/delete", galleriesC.Delete)
			r.Post("/{id}/images/{filename}/delete", galleriesC.DeleteImage)
			r.Post("/{id}/images", galleriesC.UploadImage)
		})
	})
	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Page not found", http.StatusNotFound)
	})

	// Start the server
	fmt.Printf("Starting the server on %s...\n", cfg.Server.Address)
	err = http.ListenAndServe(cfg.Server.Address, r)
	if err != nil {
		panic(err)
	}
}

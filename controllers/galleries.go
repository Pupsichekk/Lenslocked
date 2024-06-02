package controllers

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"

	"github.com/Pupsichekk/lenslocked/context"
	"github.com/Pupsichekk/lenslocked/models"
	"github.com/go-chi/chi/v5"
)

type Galleries struct {
	Templates struct {
		Show  Template
		New   Template
		Edit  Template
		Index Template
	}
	GalleryService *models.GalleryService
}

type Image struct {
	GalleryID       int
	Filename        string
	FilenameEscaped string
}

func (g Galleries) New(w http.ResponseWriter, r *http.Request) {
	var data struct {
		Title string
	}
	data.Title = r.FormValue("title")
	g.Templates.New.Execute(w, r, data)
}

func (g Galleries) Create(w http.ResponseWriter, r *http.Request) {
	var data struct {
		Title  string
		UserID int
	}
	data.UserID = context.User(r.Context()).ID
	data.Title = r.FormValue("title")
	gallery, err := g.GalleryService.Create(data.Title, data.UserID)
	if err != nil {
		g.Templates.New.Execute(w, r, data, err)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/galleries/%d/edit", gallery.ID), http.StatusFound)
}

func (g Galleries) Show(w http.ResponseWriter, r *http.Request) {
	gallery, err := g.galleryByID(w, r)
	if err != nil {
		return
	}
	var data struct {
		ID     int
		Title  string
		Images []Image
	}
	data.ID = gallery.ID
	data.Title = gallery.Title
	data.Images, err = g.imagesByID(gallery.ID)
	if err != nil {
		http.Error(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
	g.Templates.Show.Execute(w, r, data)
}

func (g Galleries) Edit(w http.ResponseWriter, r *http.Request) {
	gallery, err := g.galleryByID(w, r, userMustOwnGallery)
	if err != nil {
		return
	}
	var data struct {
		ID     int
		Title  string
		Images []Image
	}
	data.ID = gallery.ID
	data.Title = gallery.Title
	data.Images, err = g.imagesByID(gallery.ID)
	if err != nil {
		fmt.Println(err)
		http.Error(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
	g.Templates.Edit.Execute(w, r, data)
}

func (g Galleries) Update(w http.ResponseWriter, r *http.Request) {
	gallery, err := g.galleryByID(w, r, userMustOwnGallery)
	if err != nil {
		return
	}

	gallery.Title = r.FormValue("title")
	err = g.GalleryService.Update(gallery)
	if err != nil {
		http.Error(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/galleries/%d/edit", gallery.ID), http.StatusFound)
}

func (g Galleries) Index(w http.ResponseWriter, r *http.Request) {
	// TODO: Might need to check into this func further, if user is not logged in, code panics
	// with pointer to nil error
	type Gallery struct {
		ID    int
		Title string
	}
	var data struct {
		Galleries []Gallery
	}
	user := context.User(r.Context())

	galleries, err := g.GalleryService.ByUserID(user.ID)
	if err != nil {
		fmt.Println(err)
		http.Error(w, "Something went wrong", http.StatusInternalServerError)
		return
	}

	for _, gallery := range galleries {
		data.Galleries = append(data.Galleries, Gallery{
			ID:    gallery.ID,
			Title: gallery.Title,
		})
	}
	g.Templates.Index.Execute(w, r, data)
}

func (g Galleries) Delete(w http.ResponseWriter, r *http.Request) {
	gallery, err := g.galleryByID(w, r, userMustOwnGallery)
	if err != nil {
		return
	}
	err = g.GalleryService.Delete(gallery.ID)
	if err != nil {
		http.Error(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/galleries", http.StatusFound)
}

func (g Galleries) Image(w http.ResponseWriter, r *http.Request) {
	filename := g.filename(w, r)
	galleryID, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Invalid gallery ID", http.StatusNotFound)
		return
	}
	image, err := g.GalleryService.Image(galleryID, filename)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			http.Error(w, "Image not found", http.StatusNotFound)
			return
		}
		fmt.Println(err)
		http.Error(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
	http.ServeFile(w, r, image.Path)
}

func (g Galleries) UploadImage(w http.ResponseWriter, r *http.Request) {
	gallery, err := g.galleryByID(w, r, userMustOwnGallery)
	if err != nil {
		http.Error(w, "Access denied", http.StatusForbidden)
		return
	}
	err = r.ParseMultipartForm(5 * 1024 * 1024) // 5mb or 5 << 20
	if err != nil {
		http.Error(w, "Something went wrong", http.StatusInternalServerError)
		fmt.Println(err)
		return
	}
	fileHeaders := r.MultipartForm.File["images"]

	if len(fileHeaders) == 0 {
		editPath := fmt.Sprintf("/galleries/%d/edit", gallery.ID)
		http.Redirect(w, r, editPath, http.StatusFound)
	}

	for _, fileHeader := range fileHeaders {
		file, err := fileHeader.Open()
		if err != nil {
			fmt.Println(err)
			http.Error(w, "Something went wrong", http.StatusInternalServerError)
			return
		}
		defer file.Close()
		err = g.GalleryService.CreateImage(gallery.ID, fileHeader.Filename, file)
		if err != nil {
			var fileError models.FileError
			if errors.As(err, &fileError) {
				msg := fmt.Sprintf("%v has an invalid content type or extension. Only png, gif and jpg files can be uploaded", fileHeader.Filename)
				http.Error(w, msg, http.StatusBadRequest)
				return
			}
			http.Error(w, "Something went wrong", http.StatusInternalServerError)
			return
		}
		editPath := fmt.Sprintf("/galleries/%d/edit", gallery.ID)
		http.Redirect(w, r, editPath, http.StatusFound)
	}
}

func (g Galleries) DeleteImage(w http.ResponseWriter, r *http.Request) {
	filename := g.filename(w, r)
	gallery, err := g.galleryByID(w, r, userMustOwnGallery)
	if err != nil {
		http.Error(w, "Access denied", http.StatusForbidden)
		return
	}
	err = g.GalleryService.DeleteImage(gallery.ID, filename)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			http.Error(w, "Image not found", http.StatusNotFound)
			return
		}
		http.Error(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
	editPath := fmt.Sprintf("/galleries/%d/edit", gallery.ID)
	http.Redirect(w, r, editPath, http.StatusFound)
}

func (g Galleries) filename(w http.ResponseWriter, r *http.Request) string {
	filename := chi.URLParam(r, "filename")
	filename = filepath.Base(filename)
	return filename
}

type galleryOpt func(w http.ResponseWriter, r *http.Request, gallery *models.Gallery) error

func (g Galleries) galleryByID(w http.ResponseWriter, r *http.Request, opts ...galleryOpt) (*models.Gallery, error) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		return nil, err
	}
	gallery, err := g.GalleryService.ByID(id)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			http.Error(w, models.ErrNotFound.Error(), http.StatusNotFound)
			return nil, err
		}
		http.Error(w, "Something went wrong", http.StatusInternalServerError)
		return nil, err
	}
	for _, opt := range opts {
		err = opt(w, r, gallery)
		if err != nil {
			return nil, err
		}
	}
	return gallery, nil
}

func userMustOwnGallery(w http.ResponseWriter, r *http.Request, gallery *models.Gallery) error {
	user := context.User(r.Context())
	if gallery.UserID != user.ID {
		return fmt.Errorf("user does not have access to this gallery")
	}
	return nil
}

func (g Galleries) imagesByID(galleryID int) ([]Image, error) {
	convertedImages := []Image{}
	images, err := g.GalleryService.Images(galleryID)
	if err != nil {
		return nil, err
	}
	for _, image := range images {
		convertedImages = append(convertedImages, Image{
			GalleryID:       image.GalleryID,
			Filename:        image.Filename,
			FilenameEscaped: url.PathEscape(image.Filename),
		})
	}
	return convertedImages, nil
}

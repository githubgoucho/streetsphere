package streetsphere

import (
	"archive/zip"
	"fmt"
	"html/template"
	"net/http"
	"time"

	"appengine"
)

var (
	outputTemplate = template.Must(template.ParseFiles("output.html"))
)

func init() {
	http.Handle("/upload", errorHandler(uploadHandler))
}

// uploadHandler retreives the image provided by the user, pads the image,
// generates a HTML file, then stores both files within a ZIP, which is then
// sent in the response.
func uploadHandler(c appengine.Context, w http.ResponseWriter, r *http.Request) *appError {
	fn := fmt.Sprintf("photosphere-streetview-%d", time.Now().Unix())

	err := r.ParseMultipartForm(10 << 20) // 10 MiB limit
	if err != nil {
		return appErrorf(err, "could not parse form")
	}
	if r.MultipartForm == nil {
		return appErrorf(nil, "could not parse form")
	}

	b := r.MultipartForm.File["img"]
	if len(b) < 1 {
		return appErrorf(nil, "could not find image in upload")
	}
	img := b[0]

	ir, err := img.Open()
	if err != nil {
		return appErrorf(err, "could not read image from upload")
	}

	w.Header().Add("Content-Disposition", fmt.Sprintf(`attachment;filename="%s.zip"`, fn))

	zw := zip.NewWriter(w)
	iw, err := zw.Create(fmt.Sprintf("%s/%s", fn, img.Filename))
	if err != nil{
		return appErrorf(err, "could not create image in zip")
	}

	pano, err := Pad(iw, ir)
	if err != nil {
		return appErrorf(err, "could not convert image to street view format")
	}

	hw, err := zw.Create(fmt.Sprintf("%s/streetview.html", fn))
	if err != nil {
		return appErrorf(err, "could not create index file in zip")
	}

	header := fmt.Sprintf("<!-- Generated by %s.appspot.com -->", appengine.AppID(c))
	err = outputTemplate.Execute(hw, struct {
		ImageFilename string
		Pano          *PanoOpts
		Header        template.HTML
	}{img.Filename, pano, template.HTML(header)})
	if err != nil {
		return appErrorf(err, "could not write index file")
	}

	err = zw.Close()
	if err != nil {
		return appErrorf(err, "could not create zip file")
	}
	return nil
}

func logError(c appengine.Context, msg string, err error) {
	c.Errorf("%s (%v)", msg, err)
}

type errorHandler func(appengine.Context, http.ResponseWriter, *http.Request) *appError

func (handler errorHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	if e := handler(c, w, r); e != nil {
		http.Error(w, e.Message, e.Code)
		logError(c, e.Message, e.Error)
	}
}

type appError struct {
	Error   error
	Message string
	Code    int
}

func appErrorf(err error, format string, v ...interface{}) *appError {
	return &appError{err, fmt.Sprintf(format, v...), 500}
}

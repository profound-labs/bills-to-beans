(ns bills-to-beans.bill
  (:require-macros [cljs.core.async.macros :refer [go]])
  (:require [reagent.core :as r :refer [atom]]
            [reagent.format :refer [format]]
            [reagent.session :as session]
            [secretary.core :as secretary :include-macros true]
            [reforms.reagent :include-macros true :as f]
            [reforms.validation :include-macros true :as v]
            [cljs-http.client :as http]
            [cljs.core.async :refer [<!]]
            [clojure.string :as string]))

(declare <bill-transaction-form> <bill-upload-images>)

(def default-bill {:images  [{:path nil :size nil}]

                   :transaction {:title nil
                                 :date (subs (.toISOString (js/Date.)) 0 10)
                                 :amount nil
                                 :currency "EUR"
                                 :source-account "Assets:Bank:Checking"
                                 :target-account "Expenses:Car:Gasoline"}

                   :beancount nil
                   })

(defonce bill-data (r/atom default-bill))

(def accounts  (map (fn [i] [i i])
                    ["Assets:Bank:Checking"
                     "Assets:General:Donations"
                     "Assets:Petty-Cash"
                     "Expenses:Car"
                     "Expenses:Car:Gasoline"
                     "Expenses:Gas"
                     "Expenses:General"
                     "Expenses:Phone-Internet"
                     "Expenses:Post"
                     "Expenses:Purchases"
                     "Expenses:Rent"
                     "Expenses:Tranquilidade"
                     "Expenses:Water"
                     "Expenses:Wood"]))

;; TODO
;;(defonce config-data (r/atom {:save-bill-path nil}))

(defn <bill-upload-page> []
  (let [transaction (r/cursor bill-data [:transaction])
        transaction-ui-state (r/atom {})
        title (r/cursor bill-data [:transaction :title])
        beancount (r/cursor bill-data [:beancount])
        validate-transaction! (fn []
                                (v/validate! transaction transaction-ui-state
                                             (v/present [:title] "Must have")
                                             (v/present [:date] "Must have")
                                             (v/present [:amount] "Must have")
                                             (v/present [:currency] "Must have")
                                             (v/present [:source-account] "Must have")
                                             (v/present [:target-account] "Must have")
                                             ))
        submit-bill! (fn [_]
                       (when (validate-transaction!)
                         (do
                           (go (let [response (<! (http/post
                                                  "/done"
                                                  {:json-params @bill-data}))]

                                (if (:success response)
                                  (do
                                    (reset! bill-data default-bill))
                                  ;; TODO flash error
                                  (prn (:body response))
                                  ))))))]

   (fn []
     [:div.container
      [:div.row
       [:h1.col-sm-7.col-sm-offset-3
        (if (string/blank? @title)
          "New Bill"
          @title)]]
      [:div.row
       [<bill-upload-images> bill-data]]
      [:div.row
       [<bill-transaction-form> transaction transaction-ui-state]]
      [:div.row {:style {:marginBottom "2em"}}
       [:div.col-sm-7.col-sm-offset-3
        [:button.btn.btn-primary {:on-click submit-bill!}
         [:i.fa.fa-hand-o-right]
         [:span " GO"]]]]
      [:div.row
       [:div.col-sm-7.col-sm-offset-3
        [:pre
         {:style {:color "#fff" :background-color "transparent" :border "none"}}
         @beancount]]]
      ])))

(defn out-of-image-slots? []
  (= 0 (count (remove #(not (nil? (:path %))) (:images @bill-data)))))

(defn more-images! []
  (swap! bill-data update-in [:images] (fn [coll] (conj coll {}))))

(defn update-image-data! [image file-id]
  (swap! bill-data update-in [:images file-id] (fn [_] image)))

(defn <image-input> [data file-id]
  (let [field-name (str "bill_file" file-id)
        uploading? (r/atom false)
        upload-file! (fn [e]
                       (let [file (first (array-seq (-> e .-target .-files)))]
                         (do
                           (reset! uploading? true)
                           (more-images!)
                           (go (let [response (<! (http/post
                                                  "/upload"
                                                  {:multipart-params [["file" file]]}))]

                                (if (:success response)
                                  (do
                                    (reset! uploading? false)
                                    (update-image-data! (:body response) file-id))
                                  ;; TODO flash error
                                  (prn (:body response))
                                  ))))))
        path (r/cursor data [:images file-id :path])
        size (r/cursor data [:images file-id :size])
        ]

    (fn []
      (if (nil? @path)
        (if @uploading?
          ;; Spinner when uploading
          [:span
           [:i.fa.fa-3x.fa-fw.fa-spin.fa-circle-o-notch]]

          ;; Camera icon
          [:div.col-sm-2
           [:label.image-file-upload {:for field-name}
            [:i.fa.fa-3x.fa-fw.fa-camera-retro]]
           [:input.image-input
            {:type "file"
             :id field-name
             :accept "image/*;capture=camera"
             :on-change upload-file!
             }]])

         ;; OK with file size
         [:span
          [:i.fa.fa-3x.fa-fw.fa-image]
          (format "(%.1f kb)", (/ @size 1024))])
      )))

(defn <bill-upload-images> [data]
  (let [images (r/cursor data [:images])]
    (fn []
      [:form.form-horizontal
       [:div.form-group
        [:div.col-sm-7.col-sm-offset-3
         (map-indexed (fn [n image]
                        ^{:key (str "image" n)}
                        [<image-input> data n]) @images)]
        ]])))

(defn <bill-transaction-form> [data ui-state]
  (fn []
    (f/with-options {:form {:horizontal true}}
      (v/form
       ui-state
       (v/text "Title" data [:title])
       (v/date "Date" data [:date])
       (v/number "Amount" data [:amount] :placeholder "4.95")
       (v/select "Currency" data [:currency] [["EUR" "€"] ["GBP" "£"] ["USD" "$"]])
       (v/select "From" data [:source-account] accounts)
       (v/select "To" data [:target-account] accounts)
       ))
      ))





(ns bills-to-beans.documents
  (:require-macros [cljs.core.async.macros :refer [go]])
  (:require [reagent.core :as r :refer [atom]]
            [reagent.format :refer [format]]
            [reagent.session :as session]
            [secretary.core :as secretary :include-macros true]
            [reforms.reagent :include-macros true :as f]
            [reforms.validation :include-macros true :as v]
            [bills-to-beans.helpers :refer [flash!]]
            [cljs-http.client :as http]
            [cljs.core.async :refer [<!]]
            [clojure.string :as string]))

(defn out-of-document-slots? [data]
  (= 0 (count (remove #(not (nil? (:filename %))) (:documents @data)))))

(defn more-documents! [data]
  (swap! data update-in [:documents] (fn [coll] (conj coll {}))))

(defn update-document-data! [data document file-id]
  (swap! data update-in [:documents file-id] (fn [_] document)))

(def date-regex #"^(\d{4})-*(\d{2})-*(\d{2})")

(def amount-regex #"([0-9\.,€£\$]+) *\.[^\.]+$")

(defn get-date-from-the-beginning [filename]
  (if-let [m (first (re-seq date-regex filename))]
    (format "%s-%s-%s" (m 1) (m 2) (m 3))))

(defn get-amount-from-the-end [filename]
  (if-let [m (first (re-seq amount-regex filename))]
    (-> (m 1)
        (string/replace #"^[,€£\$]" "")
        (string/replace #"[,€£\$]$" "")
        (string/replace #"[,€£\$]" "."))))

(defn get-narration-from-the-middle [filename]
  (-> filename
      (string/replace date-regex "")
      (string/replace amount-regex "")
      (string/replace #"^[ _-]*" "")
      (string/replace #"[ _-]*$" "")))

(defn parse-filename! [data filename]
  (if-let [date (get-date-from-the-beginning filename)]
    (swap! data assoc :date date))
  (if-let [amount (get-amount-from-the-end filename)]
    (do
      (swap! data update-in [:postings 0 :amount] (fn [_] (* -1 amount)))
      (swap! data update-in [:postings 1 :amount] (fn [_] amount))))
  (if-let [narration (get-narration-from-the-middle filename)]
    (swap! data assoc :narration narration)))

(defn <document-input> [data file-id]
  (let [field-name (str "document_file" file-id)
        uploading? (r/atom false)
        upload-file! (fn [e]
                       (let [file (first (array-seq (-> e .-target .-files)))]
                         (do
                           (reset! uploading? true)
                           (more-documents! data)
                           (go (let [response (<! (http/post
                                                  "/upload"
                                                  {:multipart-params [["file" file]]}))]

                                (if (:success response)
                                  (let [document (:body response)]
                                    (reset! uploading? false)
                                    (update-document-data! data document file-id)
                                    (parse-filename! data (:filename document)))
                                  (flash! response)
                                  ))))))
        filename (r/cursor data [:documents file-id :filename])
        size (r/cursor data [:documents file-id :size])]

    (fn []
      (if (nil? @filename)
        (if @uploading?
          ;; Spinner when uploading
          [:tr
           [:td [:span
                 [:i.fa.fa-fw.fa-spin.fa-circle-o-notch]]]
           [:td]]

          ;; Upload button
          [:tr
           [:td
            [:button.btn.btn-primary {:style {:padding "0px"}
                                      ;; TODO click label
                                      :on-click (fn [e] (prn "click label"))}
             [:label.document-file-upload {:for field-name :style {:margin "2px"}}
              [:i.fa.fa-2x.fa-fw.fa-file]]]
            [:input.file-input
             {:type "file"
              :id field-name
              :accept "image/*;capture=camera"
              :on-change upload-file!
              }]]
           [:td]])

         ;; File details
        [:tr
         [:td [:span @filename]]
         [:td [:span (format "(%.1f kb)", (/ @size 1024))]]]
         )
      )))

(defn <document-upload> [data]
  (let [documents (r/cursor data [:documents])]
    (fn []
      [:table.table
       [:tbody
        (map-indexed (fn [n doc]
                       ^{:key (str "doc" n)}
                       [<document-input> data n]) @documents)
        ]])))


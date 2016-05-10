(ns bills-to-beans.documents
  (:require-macros [cljs.core.async.macros :refer [go]])
  (:require [reagent.core :as r :refer [atom]]
            [reagent.format :refer [format]]
            [reagent.session :as session]
            [secretary.core :as secretary :include-macros true]
            [reforms.reagent :include-macros true :as f]
            [reforms.validation :include-macros true :as v]
            [dommy.core :refer-macros [sel sel1]]
            [bills-to-beans.helpers :refer [flash! fire! filesize-str]]
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

;; TODO
(defn document-fill-missing-date [document data]
  document)

;; TODO
(defn document-fill-missing-account [document data]
  document)

(defn document-fill-missing [document data]
  (-> document
      (document-fill-missing-date data)
      (document-fill-missing-account data)))

(defn <document-input> [data file-id]
  (let [field-name (str "document_file" file-id)
        uploading? (r/atom false)
        have-already? (fn [file] (> (count (remove
                                            #(not (= (.-name file) (:filename %)))
                                            (:documents @data)))
                                    0))
        upload-file! (fn [e]
                       (let [file (first (array-seq (-> e .-target .-files)))]
                         (if (have-already? file)
                           (flash! {:body {:flash (format "Already uploaded: %s" (.-name file))}})
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
                                       (parse-filename! (r/cursor data [:transactions 0 :data]) (:filename document)))
                                     (flash! response)
                                     )))))))

        remove-document! (fn [file-id] (do (swap! data assoc-in [:documents file-id] nil)
                                           (swap! data update :documents #(into [] (remove nil? %)))))

        filename (r/cursor data [:documents file-id :filename])
        size (r/cursor data [:documents file-id :size])]

    (fn []
      (if (nil? @filename)
        (if @uploading?
          ;; Spinner when uploading
          [:tr
           [:td [:span
                 [:i.fa.fa-fw.fa-spin.fa-circle-o-notch]]]
           [:td]
           [:td]]

          ;; Upload button
          [:tr
           [:td
            [:div.document-file-upload
             [:button.btn.btn-primary {:on-click (fn [e]
                                                   (do (fire! (sel1 (str "#" field-name)) :click)
                                                       (.stopPropagation e)))}
              [:label {:for field-name
                       :on-click (fn [e]
                                   (do (.preventDefault e)
                                       (fire! (sel1 (str "#" field-name)) :click)
                                       (.stopPropagation e)))}
               [:i.fa.fa-2x.fa-fw.fa-file]]]]
            [:input.file-input
             {:type "file"
              :id field-name
              :accept "image/*;capture=camera"
              :on-change upload-file!
              }]]
           [:td]
           [:td]])

         ;; File details
        [:tr
         [:td [:span @filename]]
         [:td {:style {:textAlign "right"}}
          [:span (filesize-str @size)]]
         [:td {:style {:textAlign "right"}}
          [:button.btn.btn-danger {:on-click (fn [_] (remove-document! file-id))}
           [:i.fa.fa-remove]]]]
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


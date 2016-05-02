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
                                  (do
                                    (reset! uploading? false)
                                    (update-document-data! data (:body response) file-id))
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
            [:label.document-file-upload {:for field-name}
             [:i.fa.fa-fw.fa-file]]
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


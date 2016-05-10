(ns bills-to-beans.notes
  (:require-macros [cljs.core.async.macros :refer [go]])
  (:require [reagent.core :as r :refer [atom]]
            [reagent.format :refer [format]]
            [reagent.session :as session]
            [secretary.core :as secretary :include-macros true]
            [reforms.reagent :include-macros true :as f]
            [reforms.validation :include-macros true :as v]
            [bills-to-beans.helpers
             :refer [not-zero? first-assets-account first-expenses-account]]
            [cljs-http.client :as http]
            [cljs.core.async :refer [<!]]
            [clojure.string :as string]))

(defonce default-note
  (r/atom {:date (subs (.toISOString (js/Date.)) 0 10)
           :account ""
           :description ""}))

(defn set-accounts [data accounts]
  (swap! data assoc :account (first-assets-account accounts)))

(defn validate-note! [data ui-state]
  (v/validate! data ui-state
               (v/present [:account] "Must have")
               (v/present [:date] "Must have")
               (not-zero? [:description] "Must have")))

(defn validate-all-notes! [data]
  (if (= 0 (count (:notes @data)))
    true
    (reduce
     (fn [a b] (and a b))
     (map-indexed
      (fn [idx _]
        (let [d (r/cursor data [:notes idx :data])
              u (r/cursor data [:notes idx :ui])]
          (validate-note! d u)))
      (:notes @data))
     )))

(defn <new-note-form> [data ui-state completions]
  (fn []
    [:div
     [:div.row
      [:div.col-sm-3
       (v/form ui-state
               (v/date "Date" data [:date]))]]
     [:div.row
      [:div.col-sm-6
       (v/form ui-state
               (v/select data [:account] (map (fn [i] [i i]) (:accounts @completions))))]]
     [:div.row
      [:div.col-sm-12
       (v/form ui-state
               (v/textarea data [:description]))]]
     ]))


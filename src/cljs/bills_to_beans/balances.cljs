(ns bills-to-beans.balances
  (:require-macros [cljs.core.async.macros :refer [go]])
  (:require [reagent.core :as r :refer [atom]]
            [reagent.format :refer [format]]
            [reagent.session :as session]
            [secretary.core :as secretary :include-macros true]
            [reforms.reagent :include-macros true :as f]
            [reforms.validation :include-macros true :as v]
            [bills-to-beans.helpers
             :refer [not-zero? first-assets-account first-expenses-account
             todayiso]]
            [cljs-http.client :as http]
            [cljs.core.async :refer [<!]]
            [clojure.string :as string]))

(defonce default-balance
  (r/atom {:date (todayiso)
           :source_account ""
           :amount "0.00"
           :currency ""}))

(defn set-accounts [data accounts]
  (swap! data assoc :source_account (first-assets-account accounts)))

(defn set-currencies [data currencies]
  (swap! data assoc :currency (first currencies)))

(defn <balance-amount> [data ui-state]
  (let [error (fn [] (first (remove (fn [item] (not (= (:korks item) #{[:amount]})))
                                    (:validation-errors @ui-state))))
        classes (fn [] (if (nil? (error)) "" "has-error"))]
    (fn []
      [:for
       [:div.form-group {:class (classes)}
        [:input.form-control {:type "number"
                              :id (str "balanceamount")
                              :placeholder "4.95"
                              :step "0.01"
                              :value (:amount @data)
                              :on-change (fn [e]
                                           (let [n (.-target.value e)]
                                             (swap! data assoc :amount n)))}]
        (if (not (nil? (error)))
          [:label.error (:error-message (error))])
        ]])))

(defn validate-balance! [data ui-state]
  (v/validate! data ui-state
               (v/present [:source_account] "Must have")
               (v/present [:date] "Must have")
               (not-zero? [:amount] "Must have")))

(defn validate-all-balances! [data]
  (if (= 0 (count (:balances @data)))
    true
    (reduce
     (fn [a b] (and a b))
     (map-indexed
      (fn [idx _]
        (let [d (r/cursor data [:balances idx :data])
              u (r/cursor data [:balances idx :ui])]
          (validate-balance! d u)))
      (:balances @data))
     )))

(defn <new-balance-form> [data ui-state completions]
  (fn []
    [:div
     [:div.row
      [:div.col-sm-3
       (v/form ui-state
               (v/date "Date" data [:date]))]]
     [:div.row
      [:div.col-sm-6
       (v/form ui-state
               (v/select data [:source_account] (map (fn [i] [i i]) (:accounts @completions))))]
      [:div.col-sm-3.col-sm-offset-1
       [<balance-amount> data ui-state]]
      [:div.col-sm-2
       (v/form ui-state
               (v/select data [:currency] (map (fn [i] [i i]) (:currencies @completions))))]]
     ]))

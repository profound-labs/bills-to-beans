(ns bills-to-beans.transactions
  (:require-macros [cljs.core.async.macros :refer [go]])
  (:require [reagent.core :as r :refer [atom]]
            [reagent.format :refer [format]]
            [reagent.session :as session]
            [secretary.core :as secretary :include-macros true]
            [reforms.reagent :include-macros true :as f]
            [reforms.validation :include-macros true :as v]
            [bills-to-beans.helpers
             :refer [not-zero? first-assets-account first-expenses-account todayiso]]
            [cljs-http.client :as http]
            [cljs.core.async :refer [<!]]
            [clojure.string :as string]))

(defonce default-transaction
  (r/atom {:date (todayiso)
           :flag "*"
           :payee nil
           :narration nil
           :tags []
           :link nil
           :postings [{:account "" :amount "-0.00" :currency ""}
                      {:account "" :amount "0.00" :currency ""}]}))

(defn set-accounts [data accounts]
  (do (swap! data assoc-in [:postings 0 :account] (first-assets-account accounts))
      (swap! data assoc-in [:postings 1 :account] (first-expenses-account accounts))))

(defn set-currencies [data currencies]
  (do (swap! data assoc-in [:postings 0 :currency] (first currencies))
      (swap! data assoc-in [:postings 1 :currency] (first currencies))))

(defn balance-two-postings! [data changed-idx]
  (when (= 2 (count (:postings @data)))
    (let [other-idx (if (= 0 changed-idx) 1 0)]
      (swap! data assoc-in
             [:postings other-idx :amount]
             (format "%.2f" (* -1 (get-in @data [:postings changed-idx :amount]))))
      (swap! data assoc-in
             [:postings other-idx :currency]
             (format "%.2f" (get-in @data [:postings changed-idx :currency])))
      )))

(defn <posting-amount> [idx data ui-state]
  (let [error (fn [] (first (remove (fn [item] (not (= (:korks item) #{[:postings idx :amount]})))
                              (:validation-errors @ui-state))))
        classes (fn [] (if (nil? (error)) "" "has-error"))]
    (fn []
      [:for
       [:div.form-group {:class (classes)}
        [:input.form-control {:type "number"
                              :id (str "postings" idx "amount")
                              :placeholder "4.95"
                              :step "0.01"
                              :value (get-in @data [:postings idx :amount])
                              :on-change (fn [e]
                                           (let [n (.-target.value e)]
                                             (swap! data assoc-in [:postings idx :amount] n)
                                             (balance-two-postings! data idx)))}]
        (if (not (nil? (error)))
          [:label.error (:error-message (error))])
        ]])))

(defn <posting> [idx data ui-state completions]
  (fn []
    [:div.row
     [:div.col-sm-6
      (v/form ui-state
              (v/select data [:postings idx :account] (map (fn [i] [i i]) (:accounts @completions))))]
     [:div.col-sm-3.col-sm-offset-1
      [<posting-amount> idx data ui-state]]
     [:div.col-sm-2
      (v/form ui-state
              (v/select data [:postings idx :currency] (map (fn [i] [i i]) (:currencies @completions))
                        :on-change (fn [_] (balance-two-postings! data idx))))]]))

(defn validate-transaction! [data ui-state]
  (v/validate! data ui-state
               (v/present [:narration] "Must have")
               (v/present [:date] "Must have")
               (not-zero? [:postings 0 :amount] "Must have")
               (not-zero? [:postings 1 :amount] "Must have")))

(defn validate-all-transactions! [data]
  (if (= 0 (count (:transactions @data)))
    true
    (reduce
     (fn [a b] (and a b))
     (map-indexed
      (fn [idx _]
        (let [d (r/cursor data [:transactions idx :data])
              u (r/cursor data [:transactions idx :ui])]
          (validate-transaction! d u)))
      (:transactions @data))
     )))

(defn <new-transaction-form> [data ui-state completions]
  (fn []
    [:div
     [:div.row
      [:div.col-sm-3
       (v/form ui-state
               (v/date "Date" data [:date]))]
      [:div.col-sm-4
       (v/form ui-state
               (v/text "Payee" data [:payee]))]
      [:div.col-sm-5
       (v/form ui-state
               (v/text "Description" data [:narration]))]]
     [:div
      (map-indexed (fn [idx _]
                     ^{:key (str "posting" idx)}
                     [<posting> idx data ui-state completions])
                   (:postings @data))]
     ]))


(ns bills-to-beans.bill
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

(declare <new-transaction-form> <payees-list>)

(defonce default-transaction {:date (subs (.toISOString (js/Date.)) 0 10)
                              :flag "*"
                              :payee nil
                              :narration nil
                              :tags []
                              :link nil
                              :postings [{:account "Assets:PT:Bank:Current" :amount "-0.00" :currency "EUR"}
                                         {:account "Expenses:General" :amount "0.00" :currency "EUR"}]
                              :documents [{:path nil :size nil}]})

(defonce transaction-data (r/atom default-transaction))

(def completions (r/atom {:payees []
                          :tags []
                          :links []}))

(def currencies (r/atom []))

(def accounts (r/atom []))

(defn first-assets-account [accounts]
  "Assets:PT:Bank:Current")

(defn first-expenses-account [accounts]
  "Expenses:General")

(defn not-zero? [korks error-message]
  (fn [cursor]
    (when (or (nil? (get-in cursor korks)) (= 0 (get-in cursor korks)))
      (v/validation-error [korks] error-message))))

(defn <new-transaction-page> []
  (let [transaction-ui-state (r/atom {})
        payee (r/cursor transaction-data [:payee])
        narration (r/cursor transaction-data [:narration])

        validate-transaction! (fn []
                                (v/validate! transaction-data transaction-ui-state
                                             (v/present [:narration] "Must have")
                                             (v/present [:date] "Must have")
                                             (not-zero? [:postings 0 :amount] "Must have")
                                             (not-zero? [:postings 1 :amount] "Must have")
                                             ))

        get-resource (fn [url data & success-callback]
                       (go (let [response (<! (http/get url))]
                             (if (:success response)
                               (let [res (:body response)]
                                 (reset! data res)
                                 (if (not (nil? success-callback))
                                   ((first success-callback) res))
                                 )
                               ;; TODO flash error
                               (prn (:body response))
                               ))))

        str-amounts (fn [postings]
                      (map (fn [p] (assoc p :amount (str (:amount p))))
                           postings))

        req-save (fn []
                   (http/post
                    "/save-transaction"
                    {:json-params
                     (update-in @transaction-data [:postings] str-amounts)}))

        submit-transaction! (fn [_]
                       (when (validate-transaction!)
                         (do
                           (go (let [response (<! (req-save))]

                                (if (:success response)
                                  (do
                                    (reset! transaction-data default-transaction)
                                    (flash! response))
                                  (flash! response)
                                  ))))))]

    (r/create-class {:component-will-mount
                     (fn []
                       (get-resource "/completions.json" completions)
                       (get-resource "/accounts.json" accounts
                                     (fn [res]
                                       (swap! transaction-data update-in
                                              [:postings 0 :account] #(first-assets-account res))
                                       (swap! transaction-data update-in
                                              [:postings 1 :account] #(first-expenses-account res))))
                       (get-resource "/currencies.json" currencies
                                     (fn [res]
                                       (swap! transaction-data update-in
                                              [:postings 0 :currency] #(first @currencies))
                                       (swap! transaction-data update-in
                                              [:postings 1 :currency] #(first @currencies)))))

                     :reagent-render
                     (fn []
                       [:div.container.transaction
                        [:div.row

                         [:div.col-sm-2
                          [:h4 "Payees"]
                          [<payees-list> completions]]

                         [:div.col-sm-10
                          [:div.row
                           [:h1.col-sm-12
                            (if (string/blank? @narration)
                              "New Transaction"
                              @narration)]]
                          [:div.row
                           [:div.col-sm-12
                            [<new-transaction-form> transaction-data transaction-ui-state]]]
                          [:div.row {:style {:marginTop "2em"}}
                           [:div.col-sm-12
                            [:button.btn.btn-primary {:on-click submit-transaction!}
                             [:i.fa.fa-hand-o-right]
                             [:span " SAVE"]]]]]

                         ]]
                       )})))

(defn balance-two-postings! [changed-idx]
  (when (= 2 (count (:postings @transaction-data)))
    (let [other-idx (if (= 0 changed-idx) 1 0)]
      (swap! transaction-data assoc-in
             [:postings other-idx :amount]
             (* -1 (get-in @transaction-data [:postings changed-idx :amount])))
      (swap! transaction-data assoc-in
             [:postings other-idx :currency]
             (get-in @transaction-data [:postings changed-idx :currency]))
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
                                             (balance-two-postings! idx)))}]
        (if (not (nil? (error)))
          [:label.error (:error-message (error))])
        ]])))

(defn <posting> [idx data ui-state]
  (fn []
    [:div.row
     [:div.col-sm-6
      (v/form ui-state
              (v/select data [:postings idx :account] (map (fn [i] [i i]) @accounts)))]
     [:div.col-sm-3.col-sm-offset-1
      [<posting-amount> idx data ui-state]]
     [:div.col-sm-2
      (v/form ui-state
              (v/select data [:postings idx :currency] (map (fn [i] [i i]) @currencies)
                        :on-change (fn [_] (balance-two-postings! idx))))]]))

(defn <new-transaction-form> [data ui-state]
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
                     [<posting> idx data ui-state])
                   (:postings @data))]
     ]))

(defn <payees-list> [data]
  (fn []
    (let [set-payee! (fn [e]
                   (let [payee (.-target.innerText e)]
                     (swap! transaction-data assoc :payee payee)))]
    [:div.list-group
     (map-indexed (fn [idx p]
            ^{:key (str "payee" idx)}
                    [:button.list-group-item {:type "button" :on-click set-payee!} p])
          (:payees @data))])))



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

(declare <new-transaction-form>)

(def default-transaction {:date (subs (.toISOString (js/Date.)) 0 10)
                          :flag "*"
                          :payee nil
                          :narration nil
                          :tags []
                          :link nil
                          :postings [{:account "Assets:PT:Bank:Current" :amount nil :currency "EUR"}
                                     {:account "Expenses:General"}]
                          :documents [{:path nil :size nil}]})

(defonce transaction-data (r/atom default-transaction))

(def accounts  (map (fn [i] [i i])
                    [
                     "Assets:PT:Bank:Current"
                     "Assets:PT:Petty-Cash"
                     "Assets:UK:Bank:Current"
                     "Expenses:Car"
                     "Expenses:Car:Gasoline"
                     "Expenses:Financial:Fees"
                     "Expenses:General"
                     "Expenses:Insurance:Tranquilidade"
                     "Expenses:Maintenance"
                     "Expenses:Maintenance:Electricity"
                     "Expenses:Maintenance:Gas"
                     "Expenses:Maintenance:Rent"
                     "Expenses:Maintenance:Water"
                     "Expenses:Maintenance:Wood"
                     "Expenses:Purchases"
                     "Expenses:Travel"
                     "Expenses:Travel:Parking"
                     "Expenses:Travel:ViaVerde"
                     "Income:Donations"
                     "Income:Donations:DonationBox"
                     "Income:Donations:Retreats"
                     "Income:General"
                     ]))

;; TODO
;;(defonce config-data (r/atom {:save-bill-path nil}))

(defn <new-transaction-page> []
  (let [transaction-ui-state (r/atom {})
        payee (r/cursor transaction-data [:payee])
        narration (r/cursor transaction-data [:narration])
        validate-transaction! (fn []
                                (v/validate! transaction-data transaction-ui-state
                                             (v/present [:narration] "Must have")
                                             (v/present [:date] "Must have")
                                             ))
        submit-transaction! (fn [_]
                       (when (validate-transaction!)
                         (do
                           (go (let [response (<! (http/post
                                                  "/save-transaction"
                                                  {:json-params @transaction-data}))]

                                (if (:success response)
                                  (do
                                    (reset! transaction-data default-transaction))
                                  ;; TODO flash error
                                  (prn (:body response))
                                  ))))))]

   (fn []
     [:div.container
      [:div.row
       [:h1.col-sm-7.col-sm-offset-3
        (if (string/blank? @narration)
          "New Transaction"
          @narration)]]
      [:div.row
       [<new-transaction-form> transaction-data transaction-ui-state]]
      [:div.row {:style {:marginBottom "2em"}}
       [:div.col-sm-7.col-sm-offset-3
        [:button.btn.btn-primary {:on-click submit-transaction!}
         [:i.fa.fa-hand-o-right]
         [:span " SAVE"]]]]
      ])))

(defn <new-transaction-form> [data ui-state]
  (fn []
    (f/with-options {:form {:horizontal true}}
      (v/form
       ui-state
       (v/date "Date" data [:date])
       (v/text "Payee" data [:payee])
       (v/text "Description" data [:narration])
       ;;(v/text "Tags" data [:tags])
       ;;(v/text "Link" data [:link])
       (v/select "From" data [:postings 0 :account] accounts)
       (v/number "Amount" data [:postings 0 :amount] :placeholder "4.95")
       (v/select "Currency" data [:postings 0 :currency] [["EUR" "€"] ["GBP" "£"] ["USD" "$"]])
       (v/select "To" data [:postings 1 :account] accounts)
       ))
      ))





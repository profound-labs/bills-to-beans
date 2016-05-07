(ns bills-to-beans.bills
  (:require-macros [cljs.core.async.macros :refer [go]])
  (:require [reagent.core :as r :refer [atom]]
            [reagent.format :refer [format]]
            [reagent.session :as session]
            [secretary.core :as secretary :include-macros true]
            [reforms.reagent :include-macros true :as f]
            [reforms.validation :include-macros true :as v]
            [bills-to-beans.helpers :refer [flash! get-resource!]]
            [bills-to-beans.documents :refer [<document-upload>]]
            [bills-to-beans.transactions :refer [<new-transaction-form>
            default-transaction set-accounts set-currencies validate-all-transactions!]]
            [cljs-http.client :as http]
            [cljs.core.async :refer [<!]]
            [clojure.string :as string]))

(declare <payees-list> <notes>)

(defonce bill-data (r/atom {:transactions [{:data @default-transaction :ui {}}
                                           {:data @default-transaction :ui {}}
                                           {:data @default-transaction :ui {}}]
                            :balances []
                            :notes []
                            :documents []
                            :completions {:payees []
                                          :tags []
                                          :links []
                                          :accounts []
                                          :currencies []}}))

(defonce completions (r/cursor bill-data [:completions]))

(defn str-amounts
  "Replace all amounts with strings"
  [transactions]
  (map (fn [txn]
         (update txn :postings
                 (fn [postings]
                   (map (fn [p] (assoc p :amount (str (:amount p))))
                        postings))))
       transactions))

(defn <new-bill-page> []
  (let [req-save (fn []
                   (http/post
                    "/save-bill"
                    {:json-params
                     (-> {:transactions (:transactions @bill-data)}
                         ((fn [h] (update h :transactions (fn [a] (map #(:data %) a)))))
                         ((fn [h] (update h :transactions str-amounts))))}))

        save-bill! (fn [_]
                       (when (validate-all-transactions! bill-data)
                         (do (go (let [response (<! (req-save))]
                                   (if (:success response)
                                     (do
                                       (swap! bill-data
                                                assoc :transactions
                                                [{:data @default-transaction :ui {}}])
                                       (flash! response))
                                     (flash! response)
                                     ))))))]

    (r/create-class {:component-will-mount
                     (fn []
                       (get-resource! "/completions.json"
                                      completions
                                      (fn [res]
                                        (set-accounts default-transaction (:accounts res))
                                        (set-currencies default-transaction (:currencies res))
                                        ))
                       )

                     :reagent-render
                     (fn []
                       [:div.container.transaction
                        [:div.row

                         [:div.col-sm-2
                          [:h4 "Payees"]
                          [<payees-list> (:payees @completions)]]

                         [:div.col-sm-10

                          [:div.row
                           [:h1.col-sm-12 "New Bill"]]

                          #_[:div.row
                           [:h1.col-sm-12
                            [<document-upload> transaction-data]]]

                          (doall
                           (map-indexed
                            (fn [idx _]
                              ^{:key (str "txn" idx)}
                              [:div.row
                               [:div.col-sm-12
                                [<new-transaction-form>
                                 (r/cursor bill-data [:transactions idx :data])
                                 (r/cursor bill-data [:transactions idx :ui])
                                 completions]]])
                            (:transactions @bill-data)))

                          [:div.row {:style {:marginTop "2em"}}
                           [:div.col-sm-12
                            [:button.btn.btn-primary {:on-click save-bill!}
                             [:i.fa.fa-hand-o-right]
                             [:span " SAVE"]]]]

                          [:div.row
                           [:div.col-sm-3.pull-right
                            [<notes>]]]

                          ]

                         ]]
                       )})))

(defn <payees-list> [payees]
  (fn []
    (let [set-payee! (fn [e]
                       (prn "set payee in the active transaction")
                       #_(let [payee (.-target.innerText e)]
                         (swap! transaction-data assoc :payee payee)))]
      [:div.list-group
       (map-indexed (fn [idx p]
                      ^{:key (str "payee" idx)}
                      [:button.list-group-item {:type "button" :on-click set-payee!} p])
                    payees)])))

(defn <notes> []
  [:div
   [:p "Usually:"]
   [:table.table
    [:tbody
     [:tr [:td "- Assets"] [:td "→"] [:td "+ Expenses"]]
     [:tr [:td "- Income"] [:td "→"] [:td "+ Assets"]]
     ]]
   ])

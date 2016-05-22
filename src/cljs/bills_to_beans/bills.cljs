(ns bills-to-beans.bills
  (:require-macros [cljs.core.async.macros :refer [go]])
  (:require [reagent.core :as r :refer [atom]]
            [reagent.format :refer [format]]
            [reagent.session :as session]
            [secretary.core :as secretary :include-macros true]
            [reforms.reagent :include-macros true :as f]
            [reforms.validation :include-macros true :as v]
            [bills-to-beans.helpers
             :refer [flash! get-resource! first-assets-account
                     first-expenses-account filesize-str]]
            [bills-to-beans.payees :refer [<payees-list>]]
            [bills-to-beans.documents
             :refer [<document-upload> document-fill-missing]]
            [bills-to-beans.transactions
             :as transactions
             :refer [<new-transaction-form> default-transaction
                     validate-all-transactions!]]
            [bills-to-beans.balances
             :as balances
             :refer [<new-balance-form> default-balance validate-all-balances!]]
            [bills-to-beans.notes
             :as notes
             :refer [<new-note-form> default-note validate-all-notes!]]
            [cljs-http.client :as http]
            [cljs.core.async :refer [<!]]
            [clojure.string :as string]))

(declare <tips>)

(defonce bill-data (r/atom {:documents [{:filename nil :size nil}]
                            :transactions [{:data @default-transaction :ui {}}]
                            :balances []
                            :notes []
                            :completions {:payees []
                                          :tags []
                                          :links []
                                          :accounts []
                                          :currencies []}}))

(defonce completions (r/cursor bill-data [:completions]))

(defn str-transactions-amounts
  "Replace all transaction posting amounts with strings"
  [transactions]
  (map (fn [txn]
         (update txn :postings
                 (fn [postings]
                   (map #(assoc % :amount (str (:amount %)))
                        postings))))
       transactions))

(defn str-balances-amounts
  "Replace all balance amounts with strings"
  [balances]
  (map (fn [bal]
         (update bal :amount #(str %)))
         balances))

;; TODO fix warning about missing ^{:key}

(defn <saved-files-notice> [dir_path saved_paths saved_sizes]
  [:div
   [:p dir_path]
   [:table.table
    [:tbody
     (map-indexed
      (fn [idx a]
        ^{:key (str "files" idx)}
        [:tr
         [:td (a 0)]
         [:td (filesize-str (a 1))]])
      (map vector
           (map (fn [a] (-> a
                            (string/replace dir_path "")
                            (string/replace #"^[\/\\]+" "")))
                saved_paths)
           saved_sizes))]]])

(defn <new-bill-page> []
  (let [req-save (fn []
                   (http/post
                    "/save-bill"
                    {:json-params
                     (-> {:documents (:documents @bill-data)
                          :transactions (:transactions @bill-data)
                          :balances (:balances @bill-data)
                          :notes (:notes @bill-data)}
                         ((fn [h] (update h :documents (fn [a] (remove #(nil? (:filename %)) a)))))
                         ((fn [h] (update h :documents (fn [a] (map #(document-fill-missing % bill-data) a)))))
                         ((fn [h] (update h :transactions (fn [a] (map #(:data %) a)))))
                         ((fn [h] (update h :transactions str-transactions-amounts)))
                         ((fn [h] (update h :balances (fn [a] (map #(:data %) a)))))
                         ((fn [h] (update h :balances str-balances-amounts)))
                         ((fn [h] (update h :notes (fn [a] (map #(:data %) a)))))
                         )}))

        save-bill! (fn [_]
                     (when (and (validate-all-transactions! bill-data)
                                (validate-all-balances! bill-data)
                                (validate-all-notes! bill-data))
                         (do (go (let [response (<! (req-save))]
                                   (if (:success response)
                                     (let [notice [<saved-files-notice>
                                                   (get-in response [:body :dir_path])
                                                   (get-in response [:body :saved_paths])
                                                   (get-in response [:body :saved_sizes])]]

                                       (do (swap! bill-data assoc :documents
                                                  [{:filename nil :size nil}])
                                           (swap! bill-data assoc :transactions
                                                  [{:data @default-transaction :ui {}}])
                                           (swap! bill-data assoc :balances [])
                                           (swap! bill-data assoc :notes []))

                                       (flash! response notice))
                                     (flash! response)
                                     ))))))

        add-new-transaction! (fn [_] (swap! bill-data update :transactions
                                            (fn [a]
                                              (conj a {:data
                                                       (if (= 0 (count (:transactions @bill-data)))
                                                         @default-transaction
                                                         (assoc @default-transaction :date
                                                                (:date (:data (peek (:transactions @bill-data))))))
                                                       :ui {}}))))

        add-new-balance! (fn [_] (swap! bill-data update :balances
                                        (fn [a]
                                          (conj a {:data
                                                   (if (= 0 (count (:balances @bill-data)))
                                                     @default-balance
                                                     (assoc @default-balance :date
                                                            (:date (:data (peek (:balances @bill-data))))))
                                                   :ui {}}))))

        add-new-note! (fn [_] (swap! bill-data update :notes
                                     (fn [a]
                                       (conj a {:data
                                                (if (= 0 (count (:notes @bill-data)))
                                                  @default-note
                                                  (assoc @default-note :date
                                                         (:date (:data (peek (:notes @bill-data))))))
                                                :ui {}}))))

        remove-transaction! (fn [idx] (do (swap! bill-data assoc-in [:transactions idx] nil)
                                          (swap! bill-data update :transactions #(into [] (remove nil? %)))))

        remove-balance! (fn [idx] (do (swap! bill-data assoc-in [:balances idx] nil)
                                      (swap! bill-data update :balances #(into [] (remove nil? %)))))

        remove-note! (fn [idx] (do (swap! bill-data assoc-in [:notes idx] nil)
                                      (swap! bill-data update :notes #(into [] (remove nil? %)))))
        ]

    (r/create-class {:component-will-mount
                     (fn []
                       (get-resource! "/completions.json"
                                      completions
                                      (fn [res]
                                        (transactions/set-accounts default-transaction (:accounts res))
                                        (balances/set-accounts default-balance (:accounts res))
                                        (notes/set-accounts default-note (:accounts res))
                                        (transactions/set-currencies default-transaction (:currencies res))
                                        (balances/set-currencies default-balance (:currencies res))
                                        (swap! bill-data update :transactions
                                               (fn [a] (into [] (map #(assoc % :data @default-transaction) a))))
                                        (swap! bill-data update :balances
                                               (fn [a] (into [] (map #(assoc % :data @default-balance) a))))
                                        (swap! bill-data update :notes
                                               (fn [a] (into [] (map #(assoc % :data @default-note) a))))
                                        )))

                     :reagent-render
                     (fn []
                       [:div.container.transaction
                        [:div.row

                         [:div.col-sm-2
                          [:h4 "Payees"]
                          [<payees-list> bill-data]]

                         [:div.col-sm-10

                          [:div.row
                           [:h1 "New Bill"]]

                          [:div
                           [:div.row [:h4 "Documents"]]
                           [:div.row
                            [:div.col-sm-12
                             [<document-upload> bill-data]]]]

                          (doall
                           (map-indexed
                            (fn [idx _]
                              ^{:key (str "txn" idx)}
                              [:div [:div.row [:h4 "Transactions"]]
                               [:div.row
                                [:div.col-sm-12
                                 [<new-transaction-form>
                                  (r/cursor bill-data [:transactions idx :data])
                                  (r/cursor bill-data [:transactions idx :ui])
                                  completions]]]
                               [:div.row
                                [:div.col-sm-12 {:style {:textAlign "right"}}
                                 [:button.btn.btn-danger {:on-click (fn [_] (remove-transaction! idx))}
                                  [:i.fa.fa-remove]]]]
                               ])
                            (:transactions @bill-data)))

                          [:div.row
                           [:div.col-sm-12
                            [:button.btn.btn-default {:on-click add-new-transaction!}
                             [:i.fa.fa-plus] " Transaction"]]]

                          (doall
                           (map-indexed
                            (fn [idx _]
                              ^{:key (str "bal" idx)}
                              [:div [:div.row [:h4 "Balances"]]
                               [:div.row
                                [:div.col-sm-12
                                 [<new-balance-form>
                                  (r/cursor bill-data [:balances idx :data])
                                  (r/cursor bill-data [:balances idx :ui])
                                  completions]]]
                               [:div.row
                                [:div.col-sm-12 {:style {:textAlign "right"}}
                                 [:button.btn.btn-danger {:on-click (fn [_] (remove-balance! idx))}
                                  [:i.fa.fa-remove]]]]
                               ])
                            (:balances @bill-data)))

                          [:div.row
                           [:div.col-sm-12
                            [:button.btn.btn-default {:on-click add-new-balance!}
                             [:i.fa.fa-plus] " Balance"]]]

                          (doall
                           (map-indexed
                            (fn [idx _]
                              ^{:key (str "note" idx)}
                              [:div [:div.row [:h4 "Notes"]]
                               [:div.row
                                [:div.col-sm-12
                                 [<new-note-form>
                                  (r/cursor bill-data [:notes idx :data])
                                  (r/cursor bill-data [:notes idx :ui])
                                  completions]]]
                               [:div.row
                                [:div.col-sm-12 {:style {:textAlign "right"}}
                                 [:button.btn.btn-danger {:on-click (fn [_] (remove-note! idx))}
                                  [:i.fa.fa-remove]]]]
                               ])
                            (:notes @bill-data)))

                          [:div.row
                           [:div.col-sm-12
                            [:button.btn.btn-default {:on-click add-new-note!}
                             [:i.fa.fa-plus] " Note"]]]

                          [:div.row {:style {:marginTop "3em"}}
                           [:button.btn.btn-primary {:on-click save-bill!}
                            [:i.fa.fa-hand-o-right]
                            [:span " Save Bill"]]]

                          [:div.row
                           [:div.col-sm-3.pull-right
                            [<tips>]]]

                          ]

                         ]]
                       )})))

(defn <tips> []
  [:div
   [:p "Usually:"]
   [:table.table
    [:tbody
     [:tr [:td "- Assets"] [:td "→"] [:td "+ Expenses"]]
     [:tr [:td "- Income"] [:td "→"] [:td "+ Assets"]]
     ]]
   ])

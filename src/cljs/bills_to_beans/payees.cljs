(ns bills-to-beans.payees
  (:require [bills-to-beans.helpers :refer [flash!]]))

(defn persons-name [text]
  (first (re-find #"^(Mr|Ms|Mrs|Dr|Dra|Sr|Sra)\.* .*" text))) 

(defn <payees-list> [data]
  (let [payees (fn [] (let [all-payees (get-in @data [:completions :payees])
                            individuals (into [] (keep persons-name all-payees))
                            companies (into [] (remove persons-name all-payees))]
                        {:individuals individuals :companies companies}))
        set-payee! (fn [e]
                     (let [payee (.-target.innerText e)]
                       (do (swap! data update-in [:transactions]
                                  (fn [coll] (into [] (map #(assoc-in % [:data :payee] payee) coll))))
                           (.stopPropagation e))))
        payee-item (fn [idx p]
                     ^{:key (str "person-payee" idx)}
                     [:button.list-group-item {:type "button" :on-click set-payee!} p])]
   (fn []
     [:div.row
      [:div.col-sm-6
       [:h4 "Companies"]
       [:div.list-group
        (map-indexed payee-item (:companies (payees)))]]

      [:div.col-sm-6
       [:h4 "Individuals"]
       [:div.list-group
        (map-indexed payee-item (:individuals (payees)))]]

      ]
     )))

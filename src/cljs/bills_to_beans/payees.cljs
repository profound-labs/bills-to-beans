(ns bills-to-beans.payees
  (:require [bills-to-beans.helpers :refer [flash!]]))

(defn <payees-list> [data]
  (let [payees (fn [] (get-in @data [:completions :payees]))
        set-payee! (fn [e]
                     (let [payee (.-target.innerText e)]
                       (do (swap! data update-in [:transactions]
                                  (fn [coll] (into [] (map #(assoc-in % [:data :payee] payee) coll))))
                           (.stopPropagation e))))]
   (fn []
     [:div.list-group
      (map-indexed (fn [idx p]
                     ^{:key (str "payee" idx)}
                     [:button.list-group-item {:type "button" :on-click set-payee!} p])
                   (payees))])))

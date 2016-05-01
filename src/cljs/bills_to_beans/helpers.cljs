(ns bills-to-beans.helpers
  (:require-macros [cljs.core.async.macros :refer [go]])
  (:require [reagent.core :as r]
            [reagent.session :as session]
            [secretary.core :include-macros true :as secretary]
            [cljs-http.client :as http]
            [cljs.core.async :refer [<!]]
            [clojure.string :as string]))

(defn flash! [resp]
  (let [class (if (= 200 (:status resp)) "alert-info" "alert-warning")
        message (let [m (get-in resp [:body :flash])]
                  (if (string/blank? m) "Error" m))]
    (session/put! :flash {:class class :msg message})))

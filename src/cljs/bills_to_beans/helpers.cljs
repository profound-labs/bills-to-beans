(ns bills-to-beans.helpers
  (:require-macros [cljs.core.async.macros :refer [go]])
  (:require [reagent.core :as r]
            [reagent.session :as session]
            [secretary.core :include-macros true :as secretary]
            [reforms.reagent :include-macros true :as f]
            [reforms.validation :include-macros true :as v]
            [cljs-http.client :as http]
            [cljs.core.async :refer [<!]]
            [clojure.string :as string]))

(defn flash! [resp & notice]
  (let [class (if (= 200 (:status resp)) "alert-info" "alert-warning")
        message (let [m (get-in resp [:body :flash])]
                  (if (string/blank? m) "Error" m))]
    (session/put! :flash {:class class :msg message :notice notice})))

(defn get-resource! [url data & success-callback]
  (go (let [response (<! (http/get url))]
        (if (:success response)
          ;; Assign response to atom and run callback
          (let [res (:body response)]
            (reset! data res)
            (if (not (nil? success-callback))
              ((first success-callback) res))
            )
          ;; Flash error
          (flash! response)))))

(defn first-assets-account [accounts]
  "Assets:PT:Bank:Current")

(defn first-expenses-account [accounts]
  "Expenses:General")

(defn not-zero? [korks error-message]
  (fn [cursor]
    (let [n (get-in cursor korks)]
      (when (or (nil? n) (= n 0) (= (js/parseFloat n) 0.00))
        (v/validation-error [korks] error-message)))))

;; dommy/test/dommy/test_utils.cljs
(defn fire!
  "Creates an event of type `event-type`, optionally having
   `update-event!` mutate and return an updated event object,
   and fires it on `node`.
   Only works when `node` is in the DOM"
  [node event-type & [update-event!]]
  (let [update-event! (or update-event! identity)]
    (if (.-createEvent js/document)
      (let [event (.createEvent js/document "Event")]
        (.initEvent event (name event-type) true true)
        (.dispatchEvent node (update-event! event)))
      (.fireEvent node (str "on" (name event-type))
                  (update-event! (.createEventObject js/document))))))
